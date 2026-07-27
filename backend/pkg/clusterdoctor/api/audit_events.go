package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
	"github.com/OA879/K8Sense/backend/pkg/logger"
)

// maxAuditBodyPeek caps how much request body we buffer to find the cluster
// name — request bodies here are tiny JSON, so this is generous.
const maxAuditBodyPeek = 1 << 20

// auditPlatformScope is the cluster_id used for events that aren't tied to a
// specific cluster (licence, role, branding, notification settings).
const auditPlatformScope = "platform"

// auditMiddleware records every state-changing (and data-exporting) request to
// a /cluster-doctor endpoint in the audit log, so the log answers "who did what
// on the platform" — not just guided fixes. It is deliberately self-scoping:
// the K8sense backend shares Headlamp's router, so every request passes through
// here, but only cluster-doctor mutations/exports are recorded and everything
// else is an immediate pass-through with a single string check.
//
// Guided fixes and reverts write their own richer entries (with resource and
// revert detail), so they are excluded here to avoid double-logging.
func (s *Server) auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldAudit(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Resolve the cluster before the handler consumes the body.
		cluster := clusterFromRequest(r)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		result, errMsg := "success", ""
		if rec.status >= http.StatusBadRequest {
			result, errMsg = "failed", fmt.Sprintf("http %d", rec.status)
		}

		entry := cddb.AuditEntry{
			Actor:       actorFromRequest(r),
			Action:      auditActionName(r),
			ClusterID:   cluster,
			Result:      result,
			Error:       errMsg,
			PerformedAt: time.Now().UTC().Unix(),
		}

		if err := cddb.WriteAudit(context.Background(), s.db, entry); err != nil {
			logger.Log(logger.LevelError, map[string]string{"action": entry.Action}, err,
				"cluster-doctor: writing platform audit entry")
		}
	})
}

// clusterFromRequest finds the cluster a request targets. Most endpoints put
// it in the ?cluster= query; the scan/suppress/comment endpoints put it in the
// JSON body, which we peek and then restore so the handler still reads it.
// Falls back to the platform scope for genuinely cluster-less events.
func clusterFromRequest(r *http.Request) string {
	if c := r.URL.Query().Get("cluster"); c != "" {
		return c
	}

	if r.Body == nil {
		return auditPlatformScope
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxAuditBodyPeek))
	if err != nil {
		return auditPlatformScope
	}

	// Restore the body so the downstream handler can read it unchanged.
	r.Body = io.NopCloser(bytes.NewReader(body))

	var probe struct {
		Cluster string `json:"cluster"`
	}
	if json.Unmarshal(body, &probe) == nil && probe.Cluster != "" {
		return probe.Cluster
	}

	return auditPlatformScope
}

// shouldAudit decides whether a request is a platform event worth recording:
// any cluster-doctor mutation, plus exports (a data-access event regulated
// customers care about). Reads and the guided-fix endpoints (separately
// audited) are skipped.
func shouldAudit(r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/cluster-doctor/") {
		return false
	}

	if strings.Contains(r.URL.Path, "/guided-fix") {
		return false
	}

	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	case http.MethodGet:
		return strings.HasSuffix(r.URL.Path, "/export")
	default:
		return false
	}
}

// auditActionName renders a readable action like "PUT /rules/POD-001/toggle"
// or "POST /findings/suppress" from the request.
func auditActionName(r *http.Request) string {
	return r.Method + " /" + strings.TrimPrefix(r.URL.Path, "/cluster-doctor/")
}

// statusRecorder captures the response status so the audit entry can record
// whether the action succeeded, while otherwise behaving as the wrapped writer.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rec *statusRecorder) WriteHeader(code int) {
	if !rec.wroteHeader {
		rec.status = code
		rec.wroteHeader = true
	}

	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	rec.wroteHeader = true // an implicit 200 has now been committed
	return rec.ResponseWriter.Write(b)
}
