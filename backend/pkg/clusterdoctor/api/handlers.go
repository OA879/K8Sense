package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
)

type startScanRequest struct {
	Cluster string `json:"cluster"`
}

type startScanResponse struct {
	ScanID string `json:"scanId"`
}

// StartScan handles POST /cluster-doctor/scan. It resolves the target
// cluster's clientset synchronously (so a bad cluster name fails fast with
// a 404) but runs the scan itself in the background, returning scanId
// immediately for the frontend to open an EventSource against.
func (s *Server) StartScan(w http.ResponseWriter, r *http.Request) {
	var req startScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Cluster == "" {
		http.Error(w, `{"error": "cluster is required"}`, http.StatusBadRequest)
		return
	}

	clientset, err := s.getClient(r, req.Cluster)
	if err != nil {
		http.Error(w, `{"error": "cluster not found: `+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	scanID := uuid.NewString()
	live := newLiveScan()
	s.registerActive(scanID, live)

	go s.runScan(clientset, req.Cluster, scanID, live)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(startScanResponse{ScanID: scanID})
}

// GetFindings handles GET /cluster-doctor/findings/:scanId. Since the
// response is already a plain JSON array of Finding, this doubles as the
// ?format=json export with no extra work.
func (s *Server) GetFindings(w http.ResponseWriter, r *http.Request) {
	scanID := mux.Vars(r)["scanId"]

	findings, err := cddb.GetFindings(r.Context(), s.db, scanID)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	findings = s.enrichGuidedFix(findings)

	// Suppression/comment state is keyed by resource identity per-cluster, so
	// we need the scan's cluster. If the scan can't be read, skip suppression
	// enrichment gracefully and still return the findings.
	if scan, err := cddb.GetScan(r.Context(), s.db, scanID); err == nil {
		findings = s.enrichSuppressions(r.Context(), scan.ClusterID, findings)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(findings)
}

// ExportReport handles GET /cluster-doctor/findings/:scanId/export?format=.
// format=json streams the raw findings (same as GetFindings); format=html
// (the default) renders a self-contained, air-gap-safe HTML report as a file
// download.
func (s *Server) ExportReport(w http.ResponseWriter, r *http.Request) {
	scanID := mux.Vars(r)["scanId"]
	format := r.URL.Query().Get("format")

	findings, err := cddb.GetFindings(r.Context(), s.db, scanID)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="k8sense-report-`+scanID+`.json"`)
		_ = json.NewEncoder(w).Encode(findings)

		return
	}

	scan, err := cddb.GetScan(r.Context(), s.db, scanID)
	if err != nil {
		http.Error(w, `{"error": "scan not found"}`, http.StatusNotFound)
		return
	}

	report := clusterdoctor.BuildReportData(
		scan.ClusterID, scan.Status,
		scan.TotalFindings, scan.CriticalCount, scan.WarningCount, scan.InfoCount,
		scan.SkippedChecks, findings,
	)

	html, err := clusterdoctor.RenderHTMLReport(report)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="k8sense-report-`+scanID+`.html"`)
	_, _ = w.Write(html)
}

const defaultHistoryLimit = 50

// ListHistory handles GET /cluster-doctor/history?cluster=name.
func (s *Server) ListHistory(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	if cluster == "" {
		http.Error(w, `{"error": "cluster query param is required"}`, http.StatusBadRequest)
		return
	}

	scans, err := cddb.ListScans(r.Context(), s.db, cluster, defaultHistoryLimit)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(scans)
}

// ListRules handles GET /cluster-doctor/rules.
func (s *Server) ListRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.rules)
}

// ListAuditLog handles GET /cluster-doctor/audit-log?cluster=name.
func (s *Server) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	if cluster == "" {
		http.Error(w, `{"error": "cluster query param is required"}`, http.StatusBadRequest)
		return
	}

	entries, err := cddb.ListAudit(r.Context(), s.db, cluster, defaultHistoryLimit)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}
