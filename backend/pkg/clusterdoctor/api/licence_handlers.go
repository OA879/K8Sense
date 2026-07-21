package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/licence"
)

// GetLicence handles GET /cluster-doctor/licence — current tier, expiry,
// limits, validity.
func (s *Server) GetLicence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.currentLicence())
}

type activateRequest struct {
	// Path to a .k8sense-licence file the user browsed to, or the raw file
	// contents pasted into the activation dialog. Exactly one is used.
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ActivateLicence handles POST /cluster-doctor/licence/activate. It copies the
// supplied licence (from a path or pasted content) into the app data dir and
// re-validates. Invalid licences are rejected without being installed.
func (s *Server) ActivateLicence(w http.ResponseWriter, r *http.Request) {
	var req activateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	var data []byte

	var err error

	switch {
	case req.Content != "":
		data = []byte(req.Content)
	case req.Path != "":
		data, err = os.ReadFile(req.Path)
		if err != nil {
			http.Error(w, `{"error":"could not read licence file"}`, http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, `{"error":"path or content is required"}`, http.StatusBadRequest)
		return
	}

	// Validate before installing: write to a temp path, validate, and only
	// promote it to the real location if it checks out.
	if err := os.MkdirAll(filepath.Dir(s.licencePath), 0o755); err != nil { //nolint:mnd
		http.Error(w, `{"error":"could not prepare licence directory"}`, http.StatusInternalServerError)
		return
	}

	tmp := s.licencePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil { //nolint:mnd
		http.Error(w, `{"error":"could not stage licence"}`, http.StatusInternalServerError)
		return
	}

	if info := licence.Validate(tmp); !info.Valid {
		_ = os.Remove(tmp)
		http.Error(w, `{"error":"licence is not valid: `+info.Message+`"}`, http.StatusBadRequest)

		return
	}

	if err := os.Rename(tmp, s.licencePath); err != nil {
		_ = os.Remove(tmp)
		http.Error(w, `{"error":"could not install licence"}`, http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.reloadLicence())
}

// StartTrial handles POST /cluster-doctor/licence/trial. Generates a
// machine-bound 14-day Pro trial. Refuses if a (non-expired or expired) trial
// was already generated on this machine — trials are non-renewable.
func (s *Server) StartTrial(w http.ResponseWriter, r *http.Request) {
	if s.currentLicence().Valid {
		http.Error(w, `{"error":"a licence is already active"}`, http.StatusConflict)
		return
	}

	trialMarker := s.licencePath + ".trial-used"
	if _, err := os.Stat(trialMarker); err == nil {
		http.Error(w, `{"error":"a trial has already been used on this machine"}`, http.StatusConflict)
		return
	}

	info, err := licence.GenerateTrial(s.licencePath)
	if err != nil {
		http.Error(w, `{"error":"could not start trial: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	_ = os.WriteFile(trialMarker, []byte(info.ExpiresAt), 0o600) //nolint:mnd,errcheck

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.reloadLicence())
}
