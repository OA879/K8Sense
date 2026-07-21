package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
)

func TestGetFindingsEnrichesGuidedFix(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	scanID := env.seedScan(
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "crash-1"),
		finding("POD-008", clusterdoctor.SeverityWarning, "Pod", "demo", "nolimits-1"),
	)

	var findings []clusterdoctor.Finding
	env.decode(env.do(http.MethodGet, "/cluster-doctor/findings/"+scanID, nil), &findings)

	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2", len(findings))
	}

	byRule := map[string]clusterdoctor.Finding{}
	for _, f := range findings {
		byRule[f.RuleID] = f
	}

	// POD-001 has a guided fix in the rule set; POD-008 does not. Availability
	// is derived at read time, so this proves the enrichment path works.
	if !byRule["POD-001"].GuidedFixAvailable {
		t.Error("POD-001 should have a guided fix available")
	}

	if byRule["POD-001"].GuidedFixAction != "delete_pod" {
		t.Errorf("POD-001 action = %q, want delete_pod", byRule["POD-001"].GuidedFixAction)
	}

	if byRule["POD-008"].GuidedFixAvailable {
		t.Error("POD-008 should not have a guided fix")
	}
}

func TestSuppressionRoundTrip(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	scanID := env.seedScan(
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "crash-1"),
	)

	suppressBody := map[string]any{
		"cluster": testCluster, "ruleId": "POD-001", "namespace": "demo",
		"resourceKind": "Pod", "resourceName": "crash-1", "reason": "known issue",
	}

	if rec := env.do(http.MethodPost, "/cluster-doctor/findings/suppress", suppressBody); rec.Code != http.StatusOK {
		t.Fatalf("suppress: got %d, body %s", rec.Code, rec.Body.String())
	}

	var findings []clusterdoctor.Finding
	env.decode(env.do(http.MethodGet, "/cluster-doctor/findings/"+scanID, nil), &findings)

	if len(findings) != 1 || !findings[0].Suppressed {
		t.Fatalf("finding should be suppressed, got %+v", findings)
	}

	// Unsuppressing must restore it.
	if rec := env.do(http.MethodPost, "/cluster-doctor/findings/unsuppress", suppressBody); rec.Code != http.StatusOK {
		t.Fatalf("unsuppress: got %d", rec.Code)
	}

	env.decode(env.do(http.MethodGet, "/cluster-doctor/findings/"+scanID, nil), &findings)

	if findings[0].Suppressed {
		t.Error("finding should no longer be suppressed")
	}
}

// TestSuppressionIsScopedToResource guards the suppression key: muting one
// pod must not silently mute the same rule on a different pod.
func TestSuppressionIsScopedToResource(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	scanID := env.seedScan(
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "crash-1"),
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "crash-2"),
	)

	env.do(http.MethodPost, "/cluster-doctor/findings/suppress", map[string]any{
		"cluster": testCluster, "ruleId": "POD-001", "namespace": "demo",
		"resourceKind": "Pod", "resourceName": "crash-1", "reason": "known",
	})

	var findings []clusterdoctor.Finding
	env.decode(env.do(http.MethodGet, "/cluster-doctor/findings/"+scanID, nil), &findings)

	suppressed := map[string]bool{}
	for _, f := range findings {
		suppressed[f.ResourceName] = f.Suppressed
	}

	if !suppressed["crash-1"] {
		t.Error("crash-1 should be suppressed")
	}

	if suppressed["crash-2"] {
		t.Error("crash-2 must NOT be suppressed — suppression leaked across resources")
	}
}

func TestRuleToggleAffectsRuleList(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	rec := env.do(http.MethodPut,
		"/cluster-doctor/rules/POD-008/toggle?cluster="+testCluster+"&enabled=false", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle: got %d, body %s", rec.Code, rec.Body.String())
	}

	var rules []clusterdoctor.Rule
	env.decode(env.do(http.MethodGet, "/cluster-doctor/rules?cluster="+testCluster, nil), &rules)

	for _, r := range rules {
		if r.ID == "POD-008" && r.Enabled {
			t.Error("POD-008 should be disabled for this cluster")
		}

		if r.ID == "POD-001" && !r.Enabled {
			t.Error("POD-001 should still be enabled — toggle leaked to another rule")
		}
	}
}

func TestRuleSeverityOverrideAppliesAndClears(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	rec := env.do(http.MethodPut,
		"/cluster-doctor/rules/POD-008/severity?cluster="+testCluster+"&severity=CRITICAL", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("severity override: got %d", rec.Code)
	}

	severityOf := func(id string) string {
		var rules []clusterdoctor.Rule
		env.decode(env.do(http.MethodGet, "/cluster-doctor/rules?cluster="+testCluster, nil), &rules)

		for _, r := range rules {
			if r.ID == id {
				return r.Severity
			}
		}

		return ""
	}

	if got := severityOf("POD-008"); got != clusterdoctor.SeverityCritical {
		t.Errorf("POD-008 severity = %q, want CRITICAL", got)
	}

	// Clearing the override reverts to the rule's built-in severity.
	env.do(http.MethodPut,
		"/cluster-doctor/rules/POD-008/severity?cluster="+testCluster+"&severity=", nil)

	if got := severityOf("POD-008"); got != clusterdoctor.SeverityWarning {
		t.Errorf("POD-008 severity after clear = %q, want WARNING", got)
	}
}

func TestRuleSeverityRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	rec := env.do(http.MethodPut,
		"/cluster-doctor/rules/POD-008/severity?cluster="+testCluster+"&severity=SEVERE", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid severity: got %d, want 400", rec.Code)
	}
}

func TestExportHTMLIsSelfContained(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.grantPro()

	scanID := env.seedScan(
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "crash-1"),
	)

	rec := env.do(http.MethodGet, "/cluster-doctor/findings/"+scanID+"/export?format=html", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("export: got %d, body %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()

	if !strings.Contains(body, "crash-1") {
		t.Error("report should contain the finding's resource name")
	}

	// Air-gap requirement: the report must make zero external requests.
	for _, forbidden := range []string{
		"http://", "https://", "//cdn", "fonts.googleapis", "<script src=",
	} {
		// The report legitimately contains an xmlns-free doctype and inline
		// CSS only; any absolute URL would be an external fetch.
		if strings.Contains(body, forbidden) {
			t.Errorf("report contains external reference %q — breaks air-gapped use", forbidden)
		}
	}
}

func TestExportJSONReturnsFindings(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.grantPro()

	scanID := env.seedScan(
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "crash-1"),
	)

	rec := env.do(http.MethodGet, "/cluster-doctor/findings/"+scanID+"/export?format=json", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("json export: got %d", rec.Code)
	}

	var findings []clusterdoctor.Finding
	if err := json.Unmarshal(rec.Body.Bytes(), &findings); err != nil {
		t.Fatalf("json export is not valid JSON: %v", err)
	}

	if len(findings) != 1 {
		t.Errorf("got %d findings, want 1", len(findings))
	}
}

func TestHistoryListsScansNewestFirst(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	first := env.seedScan(finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "a"))
	second := env.seedScan(finding("POD-008", clusterdoctor.SeverityWarning, "Pod", "demo", "b"))

	var scans []struct {
		ID            string `json:"id"`
		TotalFindings int    `json:"totalFindings"`
		CriticalCount int    `json:"criticalCount"`
	}

	env.decode(env.do(http.MethodGet, "/cluster-doctor/history?cluster="+testCluster, nil), &scans)

	if len(scans) != 2 {
		t.Fatalf("got %d scans, want 2", len(scans))
	}

	if scans[0].ID != second || scans[1].ID != first {
		t.Errorf("history order = [%s %s], want newest (%s) first", scans[0].ID, scans[1].ID, second)
	}

	if scans[1].CriticalCount != 1 {
		t.Errorf("first scan criticalCount = %d, want 1", scans[1].CriticalCount)
	}
}

func TestScanDiffClassifiesFindings(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	previous := env.seedScan(
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "stays"),
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "goes"),
	)
	current := env.seedScan(
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "stays"),
		finding("POD-001", clusterdoctor.SeverityCritical, "Pod", "demo", "arrives"),
	)

	var diff struct {
		Added     []clusterdoctor.Finding `json:"added"`
		Resolved  []clusterdoctor.Finding `json:"resolved"`
		Persisted []clusterdoctor.Finding `json:"persisted"`
	}

	env.decode(env.do(http.MethodGet,
		"/cluster-doctor/findings/"+current+"/diff/"+previous, nil), &diff)

	if len(diff.Added) != 1 || diff.Added[0].ResourceName != "arrives" {
		t.Errorf("added = %+v, want just 'arrives'", diff.Added)
	}

	if len(diff.Resolved) != 1 || diff.Resolved[0].ResourceName != "goes" {
		t.Errorf("resolved = %+v, want just 'goes'", diff.Resolved)
	}

	if len(diff.Persisted) != 1 || diff.Persisted[0].ResourceName != "stays" {
		t.Errorf("persisted = %+v, want just 'stays'", diff.Persisted)
	}
}

func TestNotificationConfigRoundTrip(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.grantPro()

	rec := env.do(http.MethodPut, "/cluster-doctor/notifications", map[string]any{
		"cluster": testCluster, "slackWebhook": "https://hooks.example/x",
		"notifyCritical": true, "scheduleEnabled": true, "intervalMinutes": 15,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("save notifications: got %d, body %s", rec.Code, rec.Body.String())
	}

	var got struct {
		Notifications struct {
			SlackWebhook   string `json:"slackWebhook"`
			NotifyCritical bool   `json:"notifyCritical"`
		} `json:"notifications"`
		Schedule struct {
			Enabled         bool `json:"enabled"`
			IntervalMinutes int  `json:"intervalMinutes"`
		} `json:"schedule"`
	}

	env.decode(env.do(http.MethodGet, "/cluster-doctor/notifications?cluster="+testCluster, nil), &got)

	if got.Notifications.SlackWebhook != "https://hooks.example/x" {
		t.Errorf("slack webhook = %q", got.Notifications.SlackWebhook)
	}

	if !got.Schedule.Enabled || got.Schedule.IntervalMinutes != 15 {
		t.Errorf("schedule = %+v, want enabled/15", got.Schedule)
	}
}

// TestScheduleIntervalFloor stops a typo from hammering the API server.
func TestScheduleIntervalFloor(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.grantPro()

	env.do(http.MethodPut, "/cluster-doctor/notifications", map[string]any{
		"cluster": testCluster, "notifyCritical": true,
		"scheduleEnabled": true, "intervalMinutes": 1,
	})

	var got struct {
		Schedule struct {
			IntervalMinutes int `json:"intervalMinutes"`
		} `json:"schedule"`
	}

	env.decode(env.do(http.MethodGet, "/cluster-doctor/notifications?cluster="+testCluster, nil), &got)

	if got.Schedule.IntervalMinutes < 5 {
		t.Errorf("interval = %d, want clamped to at least 5", got.Schedule.IntervalMinutes)
	}
}

func TestBrandingRoundTripAndValidation(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.grantPro()

	if rec := env.do(http.MethodPut, "/cluster-doctor/branding", map[string]any{
		"productName": "AcmeOps", "primaryColor": "#10B981",
	}); rec.Code != http.StatusOK {
		t.Fatalf("save branding: got %d, body %s", rec.Code, rec.Body.String())
	}

	var got struct {
		ProductName  string `json:"productName"`
		PrimaryColor string `json:"primaryColor"`
	}

	env.decode(env.do(http.MethodGet, "/cluster-doctor/branding", nil), &got)

	if got.ProductName != "AcmeOps" || got.PrimaryColor != "#10B981" {
		t.Errorf("branding = %+v", got)
	}

	// A remote logo would break air-gapped installs and must be rejected.
	if rec := env.do(http.MethodPut, "/cluster-doctor/branding", map[string]any{
		"logoDataUri": "https://cdn.example.com/logo.png",
	}); rec.Code != http.StatusBadRequest {
		t.Errorf("remote logo: got %d, want 400", rec.Code)
	}
}

// TestBrandingDefaultsWhenUnset proves the app shell can always render.
func TestBrandingDefaultsWhenUnset(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	var got struct {
		ProductName string `json:"productName"`
	}

	rec := env.do(http.MethodGet, "/cluster-doctor/branding", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("branding GET must always succeed, got %d", rec.Code)
	}

	env.decode(rec, &got)

	if got.ProductName != "K8sense" {
		t.Errorf("default productName = %q, want K8sense", got.ProductName)
	}
}

func TestStartScanUnknownClusterIsNotFound(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	rec := env.do(http.MethodPost, "/cluster-doctor/scan", map[string]any{"cluster": "nope"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown cluster scan: got %d, want 404", rec.Code)
	}
}

func TestHistoryRequiresCluster(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	if rec := env.do(http.MethodGet, "/cluster-doctor/history", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("history without cluster: got %d, want 400", rec.Code)
	}
}
