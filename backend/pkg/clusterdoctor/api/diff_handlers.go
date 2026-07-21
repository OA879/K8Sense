package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
)

// ScanDiff handles GET /cluster-doctor/findings/{scanId}/diff/{prevId}. It
// loads the findings for both scans and returns which findings are new,
// resolved, or persisted between the previous scan (prevId) and the current
// one (scanId).
func (s *Server) ScanDiff(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	scanID := vars["scanId"]
	prevID := vars["prevId"]

	current, err := cddb.GetFindings(r.Context(), s.db, scanID)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	previous, err := cddb.GetFindings(r.Context(), s.db, prevID)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	diff := clusterdoctor.DiffFindings(current, previous)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(diff)
}
