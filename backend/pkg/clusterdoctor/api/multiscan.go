package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// maxParallelScans bounds how many cluster scans run concurrently, per
// K8SENSE_CONTEXT.md ("Max parallel cluster scans: 5"). Extra clusters queue
// behind the semaphore.
const maxParallelScans = 5

type multiScanRequest struct {
	Clusters []string `json:"clusters"`
}

type multiScanEntry struct {
	Cluster string `json:"cluster"`
	ScanID  string `json:"scanId,omitempty"`
	Error   string `json:"error,omitempty"`
}

// StartMultiScan handles POST /cluster-doctor/scan/multi. It launches a scan
// for each requested cluster (bounded to maxParallelScans concurrent) and
// returns a scanId per cluster immediately; progress for each is followed via
// the usual per-scan SSE endpoint. A cluster that can't be resolved is
// reported with an error but doesn't block the others.
func (s *Server) StartMultiScan(w http.ResponseWriter, r *http.Request) {
	var req multiScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Clusters) == 0 {
		http.Error(w, `{"error":"clusters is required"}`, http.StatusBadRequest)
		return
	}

	sem := make(chan struct{}, maxParallelScans)
	results := make([]multiScanEntry, len(req.Clusters))

	for i, cluster := range req.Clusters {
		clientset, err := s.getClient(r, cluster)
		if err != nil {
			results[i] = multiScanEntry{Cluster: cluster, Error: "cluster not found"}
			continue
		}

		scanID := uuid.NewString()
		live := newLiveScan()
		s.registerActive(scanID, live)
		results[i] = multiScanEntry{Cluster: cluster, ScanID: scanID}

		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			s.runScan(clientset, cluster, scanID, live)
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}
