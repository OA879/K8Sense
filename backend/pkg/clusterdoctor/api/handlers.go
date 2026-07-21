package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(findings)
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
