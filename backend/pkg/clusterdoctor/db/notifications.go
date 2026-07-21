package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// NotificationConfig holds a cluster's outbound alerting settings. Webhook
// URLs are stored locally alongside the rest of the K8sense database — they
// are never transmitted anywhere except to the webhook itself.
type NotificationConfig struct {
	ClusterID      string `json:"clusterId"`
	SlackWebhook   string `json:"slackWebhook,omitempty"`
	TeamsWebhook   string `json:"teamsWebhook,omitempty"`
	NotifyCritical bool   `json:"notifyCritical"`
}

// ScanSchedule describes a cluster's recurring scan.
type ScanSchedule struct {
	ClusterID       string `json:"clusterId"`
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"intervalMinutes"`
	LastRunAt       int64  `json:"lastRunAt"`
}

// GetNotificationConfig returns a cluster's config, or a zero-value config
// with NotifyCritical=true when the cluster has no row yet.
func GetNotificationConfig(ctx context.Context, database *sql.DB, clusterID string) (NotificationConfig, error) {
	cfg := NotificationConfig{ClusterID: clusterID, NotifyCritical: true}

	var slack, teams sql.NullString

	var notify int

	err := database.QueryRowContext(ctx, `
		SELECT COALESCE(slack_webhook, ''), COALESCE(teams_webhook, ''), notify_critical
		FROM notification_config WHERE cluster_id = ?
	`, clusterID).Scan(&slack, &teams, &notify)

	if errors.Is(err, sql.ErrNoRows) {
		return cfg, nil
	}

	if err != nil {
		return cfg, fmt.Errorf("reading notification config: %w", err)
	}

	cfg.SlackWebhook = slack.String
	cfg.TeamsWebhook = teams.String
	cfg.NotifyCritical = notify == 1

	return cfg, nil
}

// SetNotificationConfig upserts a cluster's alerting settings.
func SetNotificationConfig(ctx context.Context, database *sql.DB, cfg NotificationConfig) error {
	notify := 0
	if cfg.NotifyCritical {
		notify = 1
	}

	_, err := database.ExecContext(ctx, `
		INSERT INTO notification_config (cluster_id, slack_webhook, teams_webhook, notify_critical)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(cluster_id) DO UPDATE SET
			slack_webhook = excluded.slack_webhook,
			teams_webhook = excluded.teams_webhook,
			notify_critical = excluded.notify_critical
	`, cfg.ClusterID, nullIfEmpty(cfg.SlackWebhook), nullIfEmpty(cfg.TeamsWebhook), notify)
	if err != nil {
		return fmt.Errorf("saving notification config: %w", err)
	}

	return nil
}

// GetSchedule returns a cluster's scan schedule, defaulting to disabled/60min.
func GetSchedule(ctx context.Context, database *sql.DB, clusterID string) (ScanSchedule, error) {
	sched := ScanSchedule{ClusterID: clusterID, IntervalMinutes: 60}

	var enabled int

	err := database.QueryRowContext(ctx, `
		SELECT enabled, interval_minutes, last_run_at
		FROM scan_schedules WHERE cluster_id = ?
	`, clusterID).Scan(&enabled, &sched.IntervalMinutes, &sched.LastRunAt)

	if errors.Is(err, sql.ErrNoRows) {
		return sched, nil
	}

	if err != nil {
		return sched, fmt.Errorf("reading scan schedule: %w", err)
	}

	sched.Enabled = enabled == 1

	return sched, nil
}

// SetSchedule upserts a cluster's scan schedule, preserving last_run_at.
func SetSchedule(ctx context.Context, database *sql.DB, sched ScanSchedule) error {
	enabled := 0
	if sched.Enabled {
		enabled = 1
	}

	if sched.IntervalMinutes < 5 {
		sched.IntervalMinutes = 5 // floor: don't let a typo hammer the API server
	}

	_, err := database.ExecContext(ctx, `
		INSERT INTO scan_schedules (cluster_id, enabled, interval_minutes, last_run_at)
		VALUES (?, ?, ?, COALESCE((SELECT last_run_at FROM scan_schedules WHERE cluster_id = ?), 0))
		ON CONFLICT(cluster_id) DO UPDATE SET
			enabled = excluded.enabled,
			interval_minutes = excluded.interval_minutes
	`, sched.ClusterID, enabled, sched.IntervalMinutes, sched.ClusterID)
	if err != nil {
		return fmt.Errorf("saving scan schedule: %w", err)
	}

	return nil
}

// DueSchedules returns every enabled schedule whose next run time has passed.
func DueSchedules(ctx context.Context, database *sql.DB, now int64) ([]ScanSchedule, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT cluster_id, enabled, interval_minutes, last_run_at
		FROM scan_schedules
		WHERE enabled = 1 AND (last_run_at + interval_minutes * 60) <= ?
	`, now)
	if err != nil {
		return nil, fmt.Errorf("querying due schedules: %w", err)
	}
	defer rows.Close()

	var due []ScanSchedule

	for rows.Next() {
		var s ScanSchedule

		var enabled int

		if err := rows.Scan(&s.ClusterID, &enabled, &s.IntervalMinutes, &s.LastRunAt); err != nil {
			return nil, fmt.Errorf("scanning schedule row: %w", err)
		}

		s.Enabled = enabled == 1
		due = append(due, s)
	}

	return due, rows.Err()
}

// MarkScheduleRun stamps a schedule's last_run_at.
func MarkScheduleRun(ctx context.Context, database *sql.DB, clusterID string, at int64) error {
	_, err := database.ExecContext(ctx, `
		INSERT INTO scan_schedules (cluster_id, enabled, interval_minutes, last_run_at)
		VALUES (?, 1, 60, ?)
		ON CONFLICT(cluster_id) DO UPDATE SET last_run_at = excluded.last_run_at
	`, clusterID, at)
	if err != nil {
		return fmt.Errorf("marking schedule run: %w", err)
	}

	return nil
}
