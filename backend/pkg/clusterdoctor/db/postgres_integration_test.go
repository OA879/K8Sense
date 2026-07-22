package db_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
)

func openRawPostgres(dsn string) (*sql.DB, error) {
	return sql.Open("postgres", dsn)
}

// TestPostgresBackendEndToEnd exercises every query function against a real
// Postgres, proving the dialect port (placeholder rebinding + BIGINT
// migrations + all the SQL) actually works on the second backend rather than
// only compiling. It is skipped unless K8SENSE_TEST_PG_DSN points at a
// throwaway Postgres — the test recreates the public schema, so never aim it
// at a database you care about.
//
//	docker run -d --name k8sense-pg -e POSTGRES_PASSWORD=k8sense \
//	  -e POSTGRES_DB=k8sense -p 55432:5432 postgres:16-alpine
//	K8SENSE_TEST_PG_DSN='postgres://postgres:k8sense@localhost:55432/k8sense?sslmode=disable' \
//	  go test ./pkg/clusterdoctor/db/ -run TestPostgresBackendEndToEnd -v
func TestPostgresBackendEndToEnd(t *testing.T) {
	dsn := os.Getenv("K8SENSE_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set K8SENSE_TEST_PG_DSN to run the Postgres integration test")
	}

	ctx := context.Background()

	// Clean slate: drop and recreate the schema so migrations run from zero.
	resetPostgresSchema(t, dsn)

	database, err := cddb.Open(dsn)
	if err != nil {
		t.Fatalf("opening postgres: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if cddb.CurrentDialect() != cddb.DialectPostgres {
		t.Fatalf("expected postgres dialect, got %q", cddb.CurrentDialect())
	}

	const cluster = "pg-cluster"

	// --- scans + findings ---
	older := clusterdoctor.ScanResult{
		ID: "s1", Cluster: cluster,
		StartedAt: time.Now().Add(-time.Hour), CompletedAt: time.Now().Add(-time.Hour),
		Status: clusterdoctor.ScanCompleted,
		Findings: []clusterdoctor.Finding{
			pgFinding("s1", "POD-001", clusterdoctor.SeverityCritical, "demo", "goes"),
			pgFinding("s1", "POD-001", clusterdoctor.SeverityCritical, "demo", "stays"),
		},
	}
	if err := cddb.SaveScan(ctx, database, &older); err != nil {
		t.Fatalf("SaveScan older: %v", err)
	}

	newer := clusterdoctor.ScanResult{
		ID: "s2", Cluster: cluster,
		StartedAt: time.Now(), CompletedAt: time.Now(),
		Status: clusterdoctor.ScanCompleted,
		Findings: []clusterdoctor.Finding{
			pgFinding("s2", "POD-001", clusterdoctor.SeverityCritical, "demo", "stays"),
			pgFinding("s2", "POD-008", clusterdoctor.SeverityWarning, "demo", "arrives"),
		},
	}
	if err := cddb.SaveScan(ctx, database, &newer); err != nil {
		t.Fatalf("SaveScan newer: %v", err)
	}

	findings, err := cddb.GetFindings(ctx, database, "s2")
	if err != nil {
		t.Fatalf("GetFindings: %v", err)
	}

	if len(findings) != 2 {
		t.Fatalf("GetFindings returned %d, want 2", len(findings))
	}

	// Severity ordering (CRITICAL before WARNING) must hold on Postgres too.
	if findings[0].Severity != clusterdoctor.SeverityCritical {
		t.Errorf("findings not severity-ordered: %+v", findings)
	}

	scan, err := cddb.GetScan(ctx, database, "s2")
	if err != nil {
		t.Fatalf("GetScan: %v", err)
	}

	if scan.CriticalCount != 1 || scan.WarningCount != 1 || scan.TotalFindings != 2 {
		t.Errorf("GetScan counts = %+v", scan)
	}

	scans, err := cddb.ListScans(ctx, database, cluster, 10)
	if err != nil {
		t.Fatalf("ListScans: %v", err)
	}

	if len(scans) != 2 || scans[0].ID != "s2" {
		t.Errorf("ListScans order wrong: %+v", scans)
	}

	// --- suppressions (keyed by resource identity) ---
	if err := cddb.AddSuppression(ctx, database, cddb.Suppression{
		ClusterID: cluster, RuleID: "POD-001", Namespace: "demo",
		ResourceKind: "Pod", ResourceName: "stays", Reason: "known",
		SuppressedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("AddSuppression: %v", err)
	}

	keys, err := cddb.GetSuppressionKeys(ctx, database, cluster)
	if err != nil {
		t.Fatalf("GetSuppressionKeys: %v", err)
	}

	if !keys[cddb.SuppressionKey("POD-001", "demo", "Pod", "stays")] {
		t.Error("suppression not found for the resource it was set on")
	}

	// --- rule overrides + severity ---
	if err := cddb.SetRuleOverride(ctx, database, cluster, "POD-013", false); err != nil {
		t.Fatalf("SetRuleOverride: %v", err)
	}

	disabled, err := cddb.GetDisabledRuleIDs(ctx, database, cluster)
	if err != nil {
		t.Fatalf("GetDisabledRuleIDs: %v", err)
	}

	if !disabled["POD-013"] {
		t.Error("POD-013 should be disabled")
	}

	if err := cddb.SetRuleSeverity(ctx, database, cluster, "POD-008", clusterdoctor.SeverityCritical); err != nil {
		t.Fatalf("SetRuleSeverity: %v", err)
	}

	overrides, err := cddb.GetSeverityOverrides(ctx, database, cluster)
	if err != nil {
		t.Fatalf("GetSeverityOverrides: %v", err)
	}

	if overrides["POD-008"] != clusterdoctor.SeverityCritical {
		t.Errorf("severity override = %q", overrides["POD-008"])
	}

	// --- schedules + notification config ---
	if err := cddb.SetSchedule(ctx, database, cddb.ScanSchedule{
		ClusterID: cluster, Enabled: true, IntervalMinutes: 30,
	}); err != nil {
		t.Fatalf("SetSchedule: %v", err)
	}

	if err := cddb.MarkScheduleRun(ctx, database, cluster, time.Now().Add(-time.Hour).Unix()); err != nil {
		t.Fatalf("MarkScheduleRun: %v", err)
	}

	due, err := cddb.DueSchedules(ctx, database, time.Now().Unix())
	if err != nil {
		t.Fatalf("DueSchedules: %v", err)
	}

	if len(due) != 1 || due[0].ClusterID != cluster {
		t.Errorf("DueSchedules = %+v, want the one overdue schedule", due)
	}

	if err := cddb.SetNotificationConfig(ctx, database, cddb.NotificationConfig{
		ClusterID: cluster, SlackWebhook: "https://hooks/x", NotifyCritical: true,
	}); err != nil {
		t.Fatalf("SetNotificationConfig: %v", err)
	}

	cfg, err := cddb.GetNotificationConfig(ctx, database, cluster)
	if err != nil {
		t.Fatalf("GetNotificationConfig: %v", err)
	}

	if cfg.SlackWebhook != "https://hooks/x" {
		t.Errorf("notification config = %+v", cfg)
	}

	// --- audit log ---
	if err := cddb.WriteAudit(ctx, database, cddb.AuditEntry{
		Actor: "olakunle@abbeymortgagebank.com", Action: "delete_pod", ClusterID: cluster,
		Namespace: "demo", ResourceName: "goes", Result: "success",
		PerformedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("WriteAudit: %v", err)
	}

	entries, err := cddb.ListAudit(ctx, database, cluster, 10)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}

	if len(entries) != 1 || entries[0].Actor != "olakunle@abbeymortgagebank.com" {
		t.Errorf("ListAudit = %+v", entries)
	}

	// --- storage stats (uses the Postgres pg_database_size branch) ---
	stats, err := cddb.GetStorageStats(ctx, database)
	if err != nil {
		t.Fatalf("GetStorageStats: %v", err)
	}

	if stats.ScanCount != 2 || stats.AuditCount != 1 || stats.DBSizeBytes <= 0 {
		t.Errorf("GetStorageStats = %+v", stats)
	}

	// --- retention (window function + cross-cluster isolation) ---
	pruned, err := cddb.PruneScans(ctx, database, 1)
	if err != nil {
		t.Fatalf("PruneScans: %v", err)
	}

	if pruned != 1 {
		t.Errorf("PruneScans pruned %d, want 1 (keep newest)", pruned)
	}

	remaining, err := cddb.ListScans(ctx, database, cluster, 10)
	if err != nil {
		t.Fatalf("ListScans after prune: %v", err)
	}

	if len(remaining) != 1 || remaining[0].ID != "s2" {
		t.Errorf("after prune kept %+v, want only the newest (s2)", remaining)
	}
}

func pgFinding(scanID, ruleID, severity, namespace, name string) clusterdoctor.Finding {
	return clusterdoctor.Finding{
		ID: scanID + "-" + name, ScanID: scanID,
		RuleID: ruleID, RuleName: ruleID + " name", Severity: severity, Category: "pods",
		Namespace: namespace, ResourceKind: "Pod", ResourceName: name,
		Description: "d", Remediation: "r", DetectedAt: time.Now(),
	}
}

// resetPostgresSchema drops and recreates the public schema so migrations run
// from a clean slate. Guarded by the same env var as the test that calls it.
func resetPostgresSchema(t *testing.T, dsn string) {
	t.Helper()

	database, err := openRawPostgres(dsn)
	if err != nil {
		t.Fatalf("connecting to reset schema: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("resetting schema: %v", err)
	}
}
