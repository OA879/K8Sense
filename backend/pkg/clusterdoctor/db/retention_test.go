package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
	cddb "github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor/db"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := cddb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

// saveScan persists a scan n seconds into the past so ordering is explicit.
func saveScan(t *testing.T, h *sql.DB, cluster, id string, ageSeconds int, findings int) {
	t.Helper()

	at := time.Now().Add(-time.Duration(ageSeconds) * time.Second)

	result := clusterdoctor.ScanResult{
		ID: id, Cluster: cluster, StartedAt: at, CompletedAt: at,
		Status: clusterdoctor.ScanCompleted,
	}

	for i := 0; i < findings; i++ {
		result.Findings = append(result.Findings, clusterdoctor.Finding{
			ID: id + "-f" + string(rune('a'+i)), ScanID: id,
			RuleID: "POD-001", RuleName: "n", Severity: clusterdoctor.SeverityCritical,
			Category: "pods", ResourceKind: "Pod", ResourceName: "p", DetectedAt: at,
		})
	}

	if err := cddb.SaveScan(context.Background(), h, &result); err != nil {
		t.Fatalf("saving scan %s: %v", id, err)
	}
}

func TestPruneScansKeepsNewestPerCluster(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)

	// 5 scans for cluster A (newest = a1), 2 for cluster B.
	for i := 1; i <= 5; i++ {
		saveScan(t, h, "cluster-a", "a"+string(rune('0'+i)), i*10, 1)
	}

	saveScan(t, h, "cluster-b", "b1", 10, 1)
	saveScan(t, h, "cluster-b", "b2", 20, 1)

	pruned, err := cddb.PruneScans(context.Background(), h, 2)
	if err != nil {
		t.Fatalf("PruneScans: %v", err)
	}

	// cluster-a loses 3 (keeps 2); cluster-b keeps both.
	if pruned != 3 {
		t.Errorf("pruned = %d, want 3", pruned)
	}

	scansA, err := cddb.ListScans(context.Background(), h, "cluster-a", 100)
	if err != nil {
		t.Fatal(err)
	}

	if len(scansA) != 2 {
		t.Errorf("cluster-a kept %d scans, want 2", len(scansA))
	}

	// Retention must keep the NEWEST, not arbitrary ones.
	if scansA[0].ID != "a1" || scansA[1].ID != "a2" {
		t.Errorf("kept %s,%s — want the two newest (a1,a2)", scansA[0].ID, scansA[1].ID)
	}

	scansB, err := cddb.ListScans(context.Background(), h, "cluster-b", 100)
	if err != nil {
		t.Fatal(err)
	}

	if len(scansB) != 2 {
		t.Errorf("cluster-b kept %d scans, want 2 — pruning leaked across clusters", len(scansB))
	}
}

// TestPruneScansDeletesFindings guards against orphaned finding rows silently
// growing the database forever.
func TestPruneScansDeletesFindings(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)
	saveScan(t, h, "c", "old", 100, 3)
	saveScan(t, h, "c", "new", 10, 2)

	if _, err := cddb.PruneScans(context.Background(), h, 1); err != nil {
		t.Fatalf("PruneScans: %v", err)
	}

	// The pruned scan's findings must be gone.
	old, err := cddb.GetFindings(context.Background(), h, "old")
	if err != nil {
		t.Fatal(err)
	}

	if len(old) != 0 {
		t.Errorf("pruned scan still has %d findings", len(old))
	}

	kept, err := cddb.GetFindings(context.Background(), h, "new")
	if err != nil {
		t.Fatal(err)
	}

	if len(kept) != 2 {
		t.Errorf("kept scan has %d findings, want 2", len(kept))
	}
}

func TestPruneScansNoopWhenUnderLimit(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)
	saveScan(t, h, "c", "s1", 10, 1)

	pruned, err := cddb.PruneScans(context.Background(), h, 10)
	if err != nil {
		t.Fatalf("PruneScans: %v", err)
	}

	if pruned != 0 {
		t.Errorf("pruned = %d, want 0", pruned)
	}
}

func TestScheduleRoundTripAndFloor(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)

	if err := cddb.SetSchedule(context.Background(), h, cddb.ScanSchedule{
		ClusterID: "c", Enabled: true, IntervalMinutes: 1,
	}); err != nil {
		t.Fatalf("SetSchedule: %v", err)
	}

	got, err := cddb.GetSchedule(context.Background(), h, "c")
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}

	if !got.Enabled {
		t.Error("schedule should be enabled")
	}

	if got.IntervalMinutes < 5 {
		t.Errorf("interval = %d, want clamped to >= 5", got.IntervalMinutes)
	}
}

func TestGetScheduleDefaultsForUnknownCluster(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)

	got, err := cddb.GetSchedule(context.Background(), h, "never-configured")
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}

	if got.Enabled {
		t.Error("an unconfigured cluster must not be scheduled")
	}
}

// TestDueSchedulesRespectsInterval is the scheduler's core decision: a cluster
// is only due once its interval has actually elapsed.
func TestDueSchedulesRespectsInterval(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)
	ctx := context.Background()
	now := time.Now().Unix()

	// Ran 10 minutes ago on a 60-minute interval → not due.
	if err := cddb.SetSchedule(ctx, h, cddb.ScanSchedule{
		ClusterID: "recent", Enabled: true, IntervalMinutes: 60,
	}); err != nil {
		t.Fatal(err)
	}

	if err := cddb.MarkScheduleRun(ctx, h, "recent", now-10*60); err != nil {
		t.Fatal(err)
	}

	// Ran 2 hours ago on a 60-minute interval → due.
	if err := cddb.SetSchedule(ctx, h, cddb.ScanSchedule{
		ClusterID: "stale", Enabled: true, IntervalMinutes: 60,
	}); err != nil {
		t.Fatal(err)
	}

	if err := cddb.MarkScheduleRun(ctx, h, "stale", now-2*60*60); err != nil {
		t.Fatal(err)
	}

	// Disabled but long overdue → must never be due.
	if err := cddb.SetSchedule(ctx, h, cddb.ScanSchedule{
		ClusterID: "off", Enabled: false, IntervalMinutes: 5,
	}); err != nil {
		t.Fatal(err)
	}

	if err := cddb.MarkScheduleRun(ctx, h, "off", now-99*60*60); err != nil {
		t.Fatal(err)
	}

	// MarkScheduleRun upserts with enabled=1, so re-assert the disabled state.
	if err := cddb.SetSchedule(ctx, h, cddb.ScanSchedule{
		ClusterID: "off", Enabled: false, IntervalMinutes: 5,
	}); err != nil {
		t.Fatal(err)
	}

	due, err := cddb.DueSchedules(ctx, h, now)
	if err != nil {
		t.Fatalf("DueSchedules: %v", err)
	}

	dueIDs := map[string]bool{}
	for _, s := range due {
		dueIDs[s.ClusterID] = true
	}

	if !dueIDs["stale"] {
		t.Error("an overdue enabled schedule should be due")
	}

	if dueIDs["recent"] {
		t.Error("a recently-run schedule must not be due")
	}

	if dueIDs["off"] {
		t.Error("a disabled schedule must never be due")
	}
}

func TestRuleOverridesRoundTrip(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)
	ctx := context.Background()

	if err := cddb.SetRuleOverride(ctx, h, "c", "POD-001", false); err != nil {
		t.Fatalf("SetRuleOverride: %v", err)
	}

	disabled, err := cddb.GetDisabledRuleIDs(ctx, h, "c")
	if err != nil {
		t.Fatalf("GetDisabledRuleIDs: %v", err)
	}

	if !disabled["POD-001"] {
		t.Error("POD-001 should be disabled")
	}

	// Overrides must be per-cluster.
	other, err := cddb.GetDisabledRuleIDs(ctx, h, "other-cluster")
	if err != nil {
		t.Fatal(err)
	}

	if other["POD-001"] {
		t.Error("override leaked to another cluster")
	}

	// Re-enabling clears it.
	if err := cddb.SetRuleOverride(ctx, h, "c", "POD-001", true); err != nil {
		t.Fatal(err)
	}

	disabled, err = cddb.GetDisabledRuleIDs(ctx, h, "c")
	if err != nil {
		t.Fatal(err)
	}

	if disabled["POD-001"] {
		t.Error("POD-001 should be enabled again")
	}
}

func TestSeverityOverrideRoundTripAndClear(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)
	ctx := context.Background()

	if err := cddb.SetRuleSeverity(ctx, h, "c", "POD-008", clusterdoctor.SeverityCritical); err != nil {
		t.Fatalf("SetRuleSeverity: %v", err)
	}

	overrides, err := cddb.GetSeverityOverrides(ctx, h, "c")
	if err != nil {
		t.Fatal(err)
	}

	if overrides["POD-008"] != clusterdoctor.SeverityCritical {
		t.Errorf("override = %q, want CRITICAL", overrides["POD-008"])
	}

	// An empty severity clears the override.
	if err := cddb.SetRuleSeverity(ctx, h, "c", "POD-008", ""); err != nil {
		t.Fatal(err)
	}

	overrides, err = cddb.GetSeverityOverrides(ctx, h, "c")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := overrides["POD-008"]; ok {
		t.Error("override should have been cleared")
	}
}

// TestSeverityOverrideDoesNotDisableRule guards a subtle coupling: both live in
// the same row, so setting a severity must not accidentally flip enabled.
func TestSeverityOverrideDoesNotDisableRule(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)
	ctx := context.Background()

	if err := cddb.SetRuleSeverity(ctx, h, "c", "POD-008", clusterdoctor.SeverityInfo); err != nil {
		t.Fatal(err)
	}

	disabled, err := cddb.GetDisabledRuleIDs(ctx, h, "c")
	if err != nil {
		t.Fatal(err)
	}

	if disabled["POD-008"] {
		t.Error("setting a severity override must not disable the rule")
	}
}

func TestNotificationConfigDefaultsAndRoundTrip(t *testing.T) {
	t.Parallel()

	h := newTestDB(t)
	ctx := context.Background()

	// An unconfigured cluster defaults to notifying on critical.
	cfg, err := cddb.GetNotificationConfig(ctx, h, "fresh")
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.NotifyCritical {
		t.Error("default NotifyCritical should be true")
	}

	if err := cddb.SetNotificationConfig(ctx, h, cddb.NotificationConfig{
		ClusterID: "c", SlackWebhook: "https://hooks/x", NotifyCritical: false,
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err = cddb.GetNotificationConfig(ctx, h, "c")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.SlackWebhook != "https://hooks/x" || cfg.NotifyCritical {
		t.Errorf("config = %+v", cfg)
	}
}
