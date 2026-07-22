package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
	"github.com/OA879/K8Sense/backend/pkg/logger"
)

// ListRulesForCluster handles GET /cluster-doctor/rules?cluster=name. It is the
// cluster-aware replacement for ListRules: when a cluster is given, each rule's
// Enabled flag reflects that cluster's overrides (a rule in the disabled set
// comes back Enabled=false); without a cluster it mirrors ListRules and returns
// the rule library as loaded.
func (s *Server) ListRulesForCluster(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")

	w.Header().Set("Content-Type", "application/json")

	if cluster == "" {
		_ = json.NewEncoder(w).Encode(s.rules)
		return
	}

	disabled, err := cddb.GetDisabledRuleIDs(r.Context(), s.db, cluster)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	overrides, err := cddb.GetSeverityOverrides(r.Context(), s.db, cluster)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Copy so the per-request Enabled/Severity overrides never mutate the
	// shared rule set.
	rules := make([]clusterdoctor.Rule, len(s.rules))
	copy(rules, s.rules)

	for i := range rules {
		rules[i].Enabled = !disabled[rules[i].ID]

		if sev, ok := overrides[rules[i].ID]; ok {
			rules[i].Severity = sev
		}
	}

	_ = json.NewEncoder(w).Encode(rules)
}

// SetRuleSeverity handles PUT /cluster-doctor/rules/{id}/severity?cluster=&severity=.
// An empty severity clears the override so the rule reverts to its default.
func (s *Server) SetRuleSeverity(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, clusterdoctor.RoleAdmin) {
		return
	}

	ruleID := mux.Vars(r)["id"]

	cluster := r.URL.Query().Get("cluster")
	if cluster == "" {
		http.Error(w, `{"error": "cluster query param is required"}`, http.StatusBadRequest)
		return
	}

	severity := r.URL.Query().Get("severity")
	if severity != "" &&
		severity != clusterdoctor.SeverityCritical &&
		severity != clusterdoctor.SeverityWarning &&
		severity != clusterdoctor.SeverityInfo {
		http.Error(w, `{"error": "severity must be CRITICAL, WARNING, INFO, or empty"}`, http.StatusBadRequest)
		return
	}

	if err := cddb.SetRuleSeverity(r.Context(), s.db, cluster, ruleID, severity); err != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": cluster, "ruleId": ruleID}, err,
			"cluster-doctor: setting rule severity override")
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
}

// ToggleRule handles PUT /cluster-doctor/rules/{id}/toggle?cluster=name&enabled=bool.
// It records the enable/disable decision for one rule on one cluster. The
// cluster query param is required; enabled defaults to false when absent or
// unparseable, matching the "off unless explicitly on" intent of a toggle.
func (s *Server) ToggleRule(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, clusterdoctor.RoleAdmin) {
		return
	}

	ruleID := mux.Vars(r)["id"]

	cluster := r.URL.Query().Get("cluster")
	if cluster == "" {
		http.Error(w, `{"error": "cluster query param is required"}`, http.StatusBadRequest)
		return
	}

	enabled := r.URL.Query().Get("enabled") == "true"

	if err := cddb.SetRuleOverride(r.Context(), s.db, cluster, ruleID, enabled); err != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": cluster, "ruleId": ruleID}, err,
			"cluster-doctor: setting rule override")
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
}
