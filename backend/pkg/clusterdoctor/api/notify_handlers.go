package api

import (
	"encoding/json"
	"net/http"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
)

// GetNotifyConfig handles GET /cluster-doctor/notifications?cluster=name,
// returning both the webhook config and the scan schedule for that cluster.
func (s *Server) GetNotifyConfig(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	if cluster == "" {
		http.Error(w, `{"error": "cluster query param is required"}`, http.StatusBadRequest)
		return
	}

	cfg, err := cddb.GetNotificationConfig(r.Context(), s.db, cluster)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	sched, err := cddb.GetSchedule(r.Context(), s.db, cluster)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"notifications": cfg, "schedule": sched})
}

type notifyUpdateRequest struct {
	Cluster         string `json:"cluster"`
	SlackWebhook    string `json:"slackWebhook"`
	TeamsWebhook    string `json:"teamsWebhook"`
	NotifyCritical  bool   `json:"notifyCritical"`
	ScheduleEnabled bool   `json:"scheduleEnabled"`
	IntervalMinutes int    `json:"intervalMinutes"`
}

// SetNotifyConfig handles PUT /cluster-doctor/notifications. Scheduled scans
// and webhook alerting are Pro features, so this is licence-gated.
func (s *Server) SetNotifyConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requirePaid(w) {
		return
	}

	if !s.requireRole(w, clusterdoctor.RoleAdmin) {
		return
	}

	var req notifyUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Cluster == "" {
		http.Error(w, `{"error": "cluster is required"}`, http.StatusBadRequest)
		return
	}

	if err := cddb.SetNotificationConfig(r.Context(), s.db, cddb.NotificationConfig{
		ClusterID:      req.Cluster,
		SlackWebhook:   req.SlackWebhook,
		TeamsWebhook:   req.TeamsWebhook,
		NotifyCritical: req.NotifyCritical,
	}); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	if err := cddb.SetSchedule(r.Context(), s.db, cddb.ScanSchedule{
		ClusterID:       req.Cluster,
		Enabled:         req.ScheduleEnabled,
		IntervalMinutes: req.IntervalMinutes,
	}); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
}

// TestNotification handles POST /cluster-doctor/notifications/test?cluster=name.
// It posts a sample alert to whichever webhooks are configured so the operator
// can confirm the plumbing works without waiting for a real critical finding.
func (s *Server) TestNotification(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	if cluster == "" {
		http.Error(w, `{"error": "cluster query param is required"}`, http.StatusBadRequest)
		return
	}

	cfg, err := cddb.GetNotificationConfig(r.Context(), s.db, cluster)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	if cfg.SlackWebhook == "" && cfg.TeamsWebhook == "" {
		http.Error(w, `{"error": "no webhook configured for this cluster"}`, http.StatusBadRequest)
		return
	}

	payload := clusterdoctor.NotificationPayload{
		Cluster: cluster,
		NewCritical: []clusterdoctor.Finding{{
			RuleID: "TEST-001", RuleName: "K8sense test notification",
			Severity: clusterdoctor.SeverityCritical, ResourceKind: "Cluster", ResourceName: cluster,
		}},
	}

	var failures []string

	if cfg.SlackWebhook != "" {
		if err := clusterdoctor.PostWebhook(
			r.Context(), cfg.SlackWebhook, clusterdoctor.SlackMessage(payload),
		); err != nil {
			failures = append(failures, "slack: "+err.Error())
		}
	}

	if cfg.TeamsWebhook != "" {
		if err := clusterdoctor.PostWebhook(
			r.Context(), cfg.TeamsWebhook, clusterdoctor.TeamsMessage(payload),
		); err != nil {
			failures = append(failures, "teams: "+err.Error())
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if len(failures) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "failed", "errors": failures})

		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
}
