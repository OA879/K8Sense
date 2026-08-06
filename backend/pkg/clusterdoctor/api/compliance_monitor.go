package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
	"github.com/OA879/K8Sense/backend/pkg/logger"
)

// Compliance drift monitoring. On every scheduled run, the cluster's compliance
// posture is snapshotted and compared to the previous snapshot; if a control has
// slipped from passing to failing (drift), an alert fires on the same webhook the
// user already configured for critical findings. This is what makes Compliance
// continuous rather than a manual, point-in-time check.

// runScheduledCompliance records a compliance snapshot for a cluster and alerts
// on drift versus the previous snapshot. Best-effort: every failure is logged and
// swallowed so a monitoring hiccup never disrupts the scan loop.
func (s *Server) runScheduledCompliance(ctx context.Context, clientset kubernetes.Interface, cluster string) {
	report, err := buildComplianceReport(ctx, clientset, "")
	if err != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": cluster}, err,
			"cluster-doctor: scheduled compliance check failed")

		return
	}

	snap := cddb.ComplianceSnapshot{
		ClusterID:       cluster,
		TakenAt:         time.Now().Unix(),
		Score:           report.Score,
		Passed:          report.Passed,
		Failed:          report.Failed,
		Total:           report.Total,
		FailingControls: failingControlIDs(report),
	}

	prev, hadPrev, err := cddb.LatestComplianceSnapshot(ctx, s.db, cluster)
	if err != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": cluster}, err,
			"cluster-doctor: reading previous compliance snapshot")
	}

	if err := cddb.SaveComplianceSnapshot(ctx, s.db, snap); err != nil {
		logger.Log(logger.LevelError, map[string]string{"cluster": cluster}, err,
			"cluster-doctor: saving compliance snapshot")

		return
	}

	// No baseline yet → nothing to compare against; don't alert on first run.
	if !hadPrev {
		return
	}

	newly := newlyFailing(prev.FailingControls, snap.FailingControls)
	if len(newly) == 0 {
		return
	}

	s.notifyComplianceDrift(ctx, cluster, prev.Score, snap.Score, newly)
}

// failingControlIDs returns the IDs of the report's failing controls.
func failingControlIDs(report complianceReport) []string {
	var ids []string

	for _, c := range report.Controls {
		if c.Status == "fail" {
			ids = append(ids, c.ID)
		}
	}

	sort.Strings(ids)

	return ids
}

// newlyFailing returns control IDs present in current but not in previous — the
// controls that drifted from passing to failing.
func newlyFailing(previous, current []string) []string {
	was := make(map[string]bool, len(previous))
	for _, id := range previous {
		was[id] = true
	}

	var out []string

	for _, id := range current {
		if !was[id] {
			out = append(out, id)
		}
	}

	return out
}

// notifyComplianceDrift posts a drift alert on the cluster's configured webhook,
// reusing the same NotifyCritical opt-in as finding alerts.
func (s *Server) notifyComplianceDrift(ctx context.Context, cluster string, prevScore, score int, newly []string) {
	cfg, err := cddb.GetNotificationConfig(ctx, s.db, cluster)
	if err != nil || !cfg.NotifyCritical {
		return
	}

	if cfg.SlackWebhook == "" && cfg.TeamsWebhook == "" {
		return
	}

	payload := clusterdoctor.ComplianceDriftPayload{
		Cluster:      cluster,
		PrevScore:    prevScore,
		Score:        score,
		NewlyFailing: newly,
	}

	if cfg.SlackWebhook != "" {
		if err := clusterdoctor.PostWebhook(ctx, cfg.SlackWebhook, clusterdoctor.ComplianceDriftSlack(payload)); err != nil {
			logger.Log(logger.LevelError, map[string]string{"cluster": cluster}, err,
				"cluster-doctor: posting Slack compliance-drift alert")
		}
	}

	if cfg.TeamsWebhook != "" {
		if err := clusterdoctor.PostWebhook(ctx, cfg.TeamsWebhook, clusterdoctor.ComplianceDriftTeams(payload)); err != nil {
			logger.Log(logger.LevelError, map[string]string{"cluster": cluster}, err,
				"cluster-doctor: posting Teams compliance-drift alert")
		}
	}
}

// ComplianceHistory handles GET /cluster-doctor/compliance/history?cluster= — the
// snapshot trend for charting.
func (s *Server) ComplianceHistory(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	if cluster == "" {
		writeJSON(w, map[string]interface{}{"snapshots": []cddb.ComplianceSnapshot{}})
		return
	}

	snaps, err := cddb.ListComplianceSnapshots(r.Context(), s.db, cluster, 50)
	if err != nil {
		http.Error(w, `{"error":"could not read compliance history"}`, http.StatusInternalServerError)
		return
	}

	if snaps == nil {
		snaps = []cddb.ComplianceSnapshot{}
	}

	writeJSON(w, map[string]interface{}{"snapshots": snaps})
}
