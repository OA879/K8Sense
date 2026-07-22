package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
)

type ruleYAMLRequest struct {
	YAML string `json:"yaml"`
}

type ruleValidateResponse struct {
	Valid bool     `json:"valid"`
	Rules []string `json:"rules,omitempty"` // rule IDs parsed
	Error string   `json:"error,omitempty"`
}

// ValidateRule handles POST /cluster-doctor/rules/validate — parse-check a
// custom rule YAML without importing it (used by the import dialog for
// inline feedback).
func (s *Server) ValidateRule(w http.ResponseWriter, r *http.Request) {
	var req ruleYAMLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	rules, err := clusterdoctor.ParseRules([]byte(req.YAML))
	if err != nil {
		_ = json.NewEncoder(w).Encode(ruleValidateResponse{Valid: false, Error: err.Error()})
		return
	}

	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}

	_ = json.NewEncoder(w).Encode(ruleValidateResponse{Valid: true, Rules: ids})
}

// ImportRule handles POST /cluster-doctor/rules/import — validate a custom
// rule YAML, persist it, and add it to the live rule set (Pro tier).
func (s *Server) ImportRule(w http.ResponseWriter, r *http.Request) {
	if !s.requirePaid(w) {
		return
	}

	var req ruleYAMLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	rules, err := clusterdoctor.ParseRules([]byte(req.YAML))
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	for _, rule := range rules {
		if err := cddb.AddCustomRule(r.Context(), s.db, rule.ID, rule.Name, req.YAML); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
	}

	s.AddRules(rules)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"result": "ok", "imported": len(rules)})
}

// ListCustomRules handles GET /cluster-doctor/rules/custom.
func (s *Server) ListCustomRules(w http.ResponseWriter, r *http.Request) {
	rules, err := cddb.ListCustomRules(r.Context(), s.db)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rules)
}

// DeleteCustomRule handles DELETE /cluster-doctor/rules/custom/{id}.
func (s *Server) DeleteCustomRule(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	if err := cddb.DeleteCustomRule(r.Context(), s.db, id); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
}
