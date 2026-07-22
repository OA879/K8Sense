package api

import (
	"encoding/json"
	"net/http"
	"time"

	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
)

type testConnResponse struct {
	Reachable  bool   `json:"reachable"`
	K8sVersion string `json:"k8sVersion,omitempty"`
	LatencyMs  int64  `json:"latencyMs,omitempty"`
	Error      string `json:"error,omitempty"`
}

// TestConnection handles GET /cluster-doctor/clusters/test?cluster=NAME. It
// resolves a clientset and calls Discovery().ServerVersion(), reporting
// reachability, the Kubernetes version, and round-trip latency.
func (s *Server) TestConnection(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	if cluster == "" {
		http.Error(w, `{"error":"cluster query param is required"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	clientset, err := s.getClient(r, cluster)
	if err != nil {
		_ = json.NewEncoder(w).Encode(testConnResponse{Reachable: false, Error: "cluster not found"})
		return
	}

	start := time.Now()

	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		_ = json.NewEncoder(w).Encode(testConnResponse{Reachable: false, Error: err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(testConnResponse{
		Reachable:  true,
		K8sVersion: version.GitVersion,
		LatencyMs:  time.Since(start).Milliseconds(),
	})
}

// GetStorageStats handles GET /cluster-doctor/storage — DB size + row counts
// for the Settings storage widget.
func (s *Server) GetStorageStats(w http.ResponseWriter, r *http.Request) {
	stats, err := cddb.GetStorageStats(r.Context(), s.db)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

type purgeRequest struct {
	KeepPerCluster int `json:"keepPerCluster"`
}

// PurgeScans handles POST /cluster-doctor/storage/purge — manual "Purge old
// scans" button in Settings. Keeps the newest keepPerCluster scans per
// cluster (default 10 if unset).
func (s *Server) PurgeScans(w http.ResponseWriter, r *http.Request) {
	var req purgeRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	keep := req.KeepPerCluster
	if keep <= 0 {
		keep = 10
	}

	pruned, err := cddb.PruneScans(r.Context(), s.db, keep)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"pruned": pruned})
}
