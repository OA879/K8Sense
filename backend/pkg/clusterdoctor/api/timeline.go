package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
)

// The "What changed?" timeline. It fuses three sources K8sense already has —
// the audit log (human/platform actions), Kubernetes events (what the cluster
// did), and the latest scan's findings (what's wrong) — into one time-ordered
// stream, filterable by namespace / resource / window. That merged story is the
// thing an on-call engineer otherwise reconstructs by hand across six kubectl
// commands.

const (
	defaultTimelineWindowMin = 60
	maxTimelineEntries       = 300
	maxTimelineAudit         = 500
)

// timelineEntry is one thing that happened, from any source.
type timelineEntry struct {
	Time         int64  `json:"time"` // unix seconds
	Type         string `json:"type"` // action | event | finding
	Level        string `json:"level"` // info | warning | critical
	Title        string `json:"title"`
	Detail       string `json:"detail,omitempty"`
	Actor        string `json:"actor,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	ResourceKind string `json:"resourceKind,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
}

type timelineResponse struct {
	Entries []timelineEntry `json:"entries"`
}

// timelineOpts filters the merged stream.
type timelineOpts struct {
	namespace string
	resource  string    // case-insensitive substring match on resource name
	since     time.Time // only entries at or after this
}

// Timeline handles GET /cluster-doctor/timeline?cluster=&namespace=&resource=&sinceMinutes= .
func (s *Server) Timeline(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")

	clientset, err := s.getClient(r, cluster)
	if err != nil {
		http.Error(w, `{"error": "cluster not found"}`, http.StatusNotFound)
		return
	}

	windowMin := defaultTimelineWindowMin
	if v, err := strconv.Atoi(r.URL.Query().Get("sinceMinutes")); err == nil && v > 0 {
		windowMin = v
	}

	opts := timelineOpts{
		namespace: r.URL.Query().Get("namespace"),
		resource:  r.URL.Query().Get("resource"),
		since:     time.Now().Add(-time.Duration(windowMin) * time.Minute),
	}

	audits, events, findings := s.gatherTimelineSources(r.Context(), clientset, cluster, opts.namespace)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(timelineResponse{Entries: mergeTimeline(audits, events, findings, opts)})
}

// gatherTimelineSources pulls the three raw sources; each is best-effort so one
// unavailable source never blanks the whole timeline.
func (s *Server) gatherTimelineSources(
	ctx context.Context, clientset kubernetes.Interface, cluster, namespace string,
) ([]cddb.AuditEntry, []corev1.Event, []clusterdoctor.Finding) {
	audits, _ := cddb.ListAudit(ctx, s.db, cluster, maxTimelineAudit)

	ns := namespace
	if ns == "" {
		ns = metav1.NamespaceAll
	}

	var events []corev1.Event
	if evs, err := clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{}); err == nil {
		events = evs.Items
	}

	// Findings come from the most recent scan for this cluster.
	var findings []clusterdoctor.Finding
	if scans, err := cddb.ListScans(ctx, s.db, cluster, 1); err == nil && len(scans) > 0 {
		findings, _ = cddb.GetFindings(ctx, s.db, scans[0].ID)
	}

	return audits, events, findings
}

// mergeTimeline is the pure core: convert, filter, and time-sort the three
// sources into a single newest-first stream. Kept free of I/O so it is unit-
// testable with injected data.
func mergeTimeline(
	audits []cddb.AuditEntry, events []corev1.Event, findings []clusterdoctor.Finding, opts timelineOpts,
) []timelineEntry {
	out := make([]timelineEntry, 0, len(audits)+len(events)+len(findings))

	for _, a := range audits {
		out = appendIfMatch(out, auditToEntry(a), opts)
	}

	for _, e := range events {
		out = appendIfMatch(out, eventToEntry(e), opts)
	}

	for _, f := range findings {
		out = appendIfMatch(out, findingToEntry(f), opts)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })

	if len(out) > maxTimelineEntries {
		out = out[:maxTimelineEntries]
	}

	return out
}

// appendIfMatch drops entries before the window or outside the namespace/
// resource filter (and skips zero-time entries that carry no useful ordering).
func appendIfMatch(out []timelineEntry, e timelineEntry, opts timelineOpts) []timelineEntry {
	if e.Time == 0 || e.Time < opts.since.Unix() {
		return out
	}

	if opts.namespace != "" && e.Namespace != opts.namespace {
		return out
	}

	if opts.resource != "" && !strings.Contains(strings.ToLower(e.ResourceName), strings.ToLower(opts.resource)) {
		return out
	}

	return append(out, e)
}

func auditToEntry(a cddb.AuditEntry) timelineEntry {
	level := "info"
	if a.Result == "failed" {
		level = "warning"
	}

	return timelineEntry{
		Time: a.PerformedAt, Type: "action", Level: level,
		Title: humanizeAction(a.Action), Detail: a.Error, Actor: a.Actor,
		Namespace: a.Namespace, ResourceKind: a.ResourceKind, ResourceName: a.ResourceName,
	}
}

func eventToEntry(e corev1.Event) timelineEntry {
	t := e.LastTimestamp.Time
	if t.IsZero() {
		t = e.EventTime.Time
	}

	level := "info"
	if e.Type == corev1.EventTypeWarning {
		level = "warning"
	}

	title := e.Reason
	if e.Count > 1 {
		title = e.Reason + " (x" + strconv.Itoa(int(e.Count)) + ")"
	}

	return timelineEntry{
		Time: t.Unix(), Type: "event", Level: level,
		Title: title, Detail: e.Message,
		Namespace:    e.InvolvedObject.Namespace,
		ResourceKind: e.InvolvedObject.Kind,
		ResourceName: e.InvolvedObject.Name,
	}
}

func findingToEntry(f clusterdoctor.Finding) timelineEntry {
	return timelineEntry{
		Time: f.DetectedAt.Unix(), Type: "finding", Level: strings.ToLower(f.Severity),
		Title: f.RuleName, Detail: f.Description,
		Namespace: f.Namespace, ResourceKind: f.ResourceKind, ResourceName: f.ResourceName,
	}
}

// humanizeAction turns an audit action code into a readable phrase.
func humanizeAction(action string) string {
	switch action {
	case "scale_deployment":
		return "Scaled deployment"
	case "restart_deployment":
		return "Restarted deployment"
	case "delete_pod":
		return "Deleted pod"
	case "delete_job":
		return "Deleted job"
	case "uncordon_node":
		return "Uncordoned node"
	case "cordon_node":
		return "Cordoned node"
	case "POST /scan", "POST /scan/multi":
		return "Ran a scan"
	}

	if strings.HasPrefix(action, "revert_") {
		return "Reverted: " + humanizeAction(strings.TrimPrefix(action, "revert_"))
	}

	return action
}
