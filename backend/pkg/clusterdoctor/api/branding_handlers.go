package api

import (
	"encoding/json"
	"net/http"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
)

// GetBranding handles GET /cluster-doctor/branding. Unauthenticated-friendly
// and ungated: the frontend needs it on every page load to render the app
// shell, and an absent config simply yields K8sense defaults.
func (s *Server) GetBranding(w http.ResponseWriter, r *http.Request) {
	branding, err := clusterdoctor.LoadBranding(s.brandingPath())
	if err != nil {
		// A broken branding file must not black out the app — fall back to
		// stock branding and report that.
		branding = clusterdoctor.Branding{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"productName":   branding.Name(),
		"primaryColor":  branding.PrimaryColor,
		"logoDataUri":   branding.LogoDataURI,
		"hidePoweredBy": branding.HidePoweredBy,
	})
}

// SetBranding handles PUT /cluster-doctor/branding. White-labelling is an
// enterprise feature and an admin-level change.
func (s *Server) SetBranding(w http.ResponseWriter, r *http.Request) {
	if !s.requirePaid(w) {
		return
	}

	if !s.requireRole(w, clusterdoctor.RoleAdmin) {
		return
	}

	var branding clusterdoctor.Branding
	if err := json.NewDecoder(r.Body).Decode(&branding); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := clusterdoctor.SaveBranding(s.brandingPath(), branding); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
}

// GetRole handles GET /cluster-doctor/role, telling the UI which actions to
// offer.
func (s *Server) GetRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"role": string(s.currentRole())})
}

// SetRole handles PUT /cluster-doctor/role. Changing the role is itself an
// admin action, so a viewer can't promote themselves through the UI.
func (s *Server) SetRole(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, clusterdoctor.RoleAdmin) {
		return
	}

	var body struct {
		Role clusterdoctor.Role `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.Role.Valid() {
		http.Error(w, `{"error":"role must be viewer, operator or admin"}`, http.StatusBadRequest)
		return
	}

	if err := clusterdoctor.SaveRole(s.rolePath(), body.Role); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"result": "ok", "role": string(body.Role)})
}
