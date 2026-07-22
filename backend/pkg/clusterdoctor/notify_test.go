package clusterdoctor_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

func f(ruleID, severity, namespace, name string) clusterdoctor.Finding {
	return clusterdoctor.Finding{
		RuleID: ruleID, RuleName: ruleID + " name", Severity: severity,
		Namespace: namespace, ResourceKind: "Pod", ResourceName: name,
	}
}

// TestNewCriticalFindingsOnlyReportsNewCriticals is the core alerting
// contract: operators must be paged for what just broke, not for the entire
// standing backlog, and never for non-critical churn.
func TestNewCriticalFindingsOnlyReportsNewCriticals(t *testing.T) {
	t.Parallel()

	previous := []clusterdoctor.Finding{
		f("POD-001", clusterdoctor.SeverityCritical, "demo", "old-crash"),
		f("POD-008", clusterdoctor.SeverityWarning, "demo", "old-warn"),
	}
	current := []clusterdoctor.Finding{
		f("POD-001", clusterdoctor.SeverityCritical, "demo", "old-crash"), // persists
		f("POD-001", clusterdoctor.SeverityCritical, "demo", "new-crash"), // NEW critical
		f("POD-008", clusterdoctor.SeverityWarning, "demo", "new-warn"),   // new, but warning
	}

	got := clusterdoctor.NewCriticalFindings(current, previous)

	if len(got) != 1 {
		t.Fatalf("got %d new criticals, want 1: %+v", len(got), got)
	}

	if got[0].ResourceName != "new-crash" {
		t.Errorf("got %q, want new-crash", got[0].ResourceName)
	}
}

func TestNewCriticalFindingsEmptyWhenNothingChanged(t *testing.T) {
	t.Parallel()

	findings := []clusterdoctor.Finding{
		f("POD-001", clusterdoctor.SeverityCritical, "demo", "crash"),
	}

	if got := clusterdoctor.NewCriticalFindings(findings, findings); len(got) != 0 {
		t.Errorf("got %d new criticals for an unchanged scan, want 0", len(got))
	}
}

// TestNewCriticalFindingsDistinguishesNamespaces guards the identity key: the
// same rule on same-named pods in different namespaces are different findings.
func TestNewCriticalFindingsDistinguishesNamespaces(t *testing.T) {
	t.Parallel()

	previous := []clusterdoctor.Finding{
		f("POD-001", clusterdoctor.SeverityCritical, "ns-a", "web"),
	}
	current := []clusterdoctor.Finding{
		f("POD-001", clusterdoctor.SeverityCritical, "ns-a", "web"),
		f("POD-001", clusterdoctor.SeverityCritical, "ns-b", "web"),
	}

	got := clusterdoctor.NewCriticalFindings(current, previous)

	if len(got) != 1 || got[0].Namespace != "ns-b" {
		t.Errorf("got %+v, want just the ns-b finding", got)
	}
}

func TestDiffFindingsClassification(t *testing.T) {
	t.Parallel()

	previous := []clusterdoctor.Finding{
		f("POD-001", clusterdoctor.SeverityCritical, "demo", "stays"),
		f("POD-001", clusterdoctor.SeverityCritical, "demo", "goes"),
	}
	current := []clusterdoctor.Finding{
		f("POD-001", clusterdoctor.SeverityCritical, "demo", "stays"),
		f("POD-001", clusterdoctor.SeverityCritical, "demo", "arrives"),
	}

	diff := clusterdoctor.DiffFindings(current, previous)

	if len(diff.Added) != 1 || diff.Added[0].ResourceName != "arrives" {
		t.Errorf("added = %+v", diff.Added)
	}

	if len(diff.Resolved) != 1 || diff.Resolved[0].ResourceName != "goes" {
		t.Errorf("resolved = %+v", diff.Resolved)
	}

	if len(diff.Persisted) != 1 || diff.Persisted[0].ResourceName != "stays" {
		t.Errorf("persisted = %+v", diff.Persisted)
	}
}

func TestDiffFindingsHandlesEmptyPrevious(t *testing.T) {
	t.Parallel()

	current := []clusterdoctor.Finding{
		f("POD-001", clusterdoctor.SeverityCritical, "demo", "a"),
	}

	diff := clusterdoctor.DiffFindings(current, nil)

	if len(diff.Added) != 1 || len(diff.Resolved) != 0 || len(diff.Persisted) != 0 {
		t.Errorf("diff against empty previous = %+v", diff)
	}
}

func TestSlackMessageShape(t *testing.T) {
	t.Parallel()

	payload := clusterdoctor.NotificationPayload{
		Cluster: "prod-1",
		NewCritical: []clusterdoctor.Finding{
			f("POD-001", clusterdoctor.SeverityCritical, "demo", "crash-1"),
		},
	}

	var body struct {
		Text string `json:"text"`
	}

	if err := json.Unmarshal(clusterdoctor.SlackMessage(payload), &body); err != nil {
		t.Fatalf("Slack message is not valid JSON: %v", err)
	}

	for _, want := range []string{"prod-1", "POD-001", "demo/crash-1", "1 new critical"} {
		if !strings.Contains(body.Text, want) {
			t.Errorf("Slack text missing %q; got %q", want, body.Text)
		}
	}
}

// TestSlackMessageTruncatesLongLists keeps a mass-failure alert from exceeding
// chat webhook body limits.
func TestSlackMessageTruncatesLongLists(t *testing.T) {
	t.Parallel()

	var findings []clusterdoctor.Finding
	for i := 0; i < 50; i++ {
		findings = append(findings, f("POD-001", clusterdoctor.SeverityCritical, "demo", "pod"))
	}

	var body struct {
		Text string `json:"text"`
	}

	if err := json.Unmarshal(
		clusterdoctor.SlackMessage(clusterdoctor.NotificationPayload{
			Cluster: "prod-1", NewCritical: findings,
		}), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if !strings.Contains(body.Text, "and 40 more") {
		t.Errorf("expected a truncation notice, got %q", body.Text)
	}
}

func TestTeamsMessageShape(t *testing.T) {
	t.Parallel()

	payload := clusterdoctor.NotificationPayload{
		Cluster: "prod-1",
		NewCritical: []clusterdoctor.Finding{
			f("POD-001", clusterdoctor.SeverityCritical, "demo", "crash-1"),
		},
	}

	var card map[string]any
	if err := json.Unmarshal(clusterdoctor.TeamsMessage(payload), &card); err != nil {
		t.Fatalf("Teams message is not valid JSON: %v", err)
	}

	// Teams rejects anything that isn't a MessageCard.
	if card["@type"] != "MessageCard" {
		t.Errorf("@type = %v, want MessageCard", card["@type"])
	}

	if !strings.Contains(card["summary"].(string), "prod-1") {
		t.Errorf("summary = %v, want it to name the cluster", card["summary"])
	}
}

func TestPostWebhookSucceedsOn2xx(t *testing.T) {
	t.Parallel()

	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := clusterdoctor.PostWebhook(context.Background(), srv.URL, []byte(`{"text":"hi"}`))
	if err != nil {
		t.Fatalf("PostWebhook: %v", err)
	}

	if !strings.Contains(gotBody, "hi") {
		t.Errorf("webhook received %q", gotBody)
	}
}

// TestPostWebhookReportsNon2xx matters because delivery is best-effort: the
// caller can only log a failure if it's actually surfaced.
func TestPostWebhookReportsNon2xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := clusterdoctor.PostWebhook(context.Background(), srv.URL, []byte(`{}`)); err == nil {
		t.Error("expected an error for a 500 response")
	}
}

func TestPostWebhookFailsOnUnreachableHost(t *testing.T) {
	t.Parallel()

	// Port 0 is never listening.
	err := clusterdoctor.PostWebhook(context.Background(), "http://127.0.0.1:0/hook", []byte(`{}`))
	if err == nil {
		t.Error("expected an error posting to an unreachable host")
	}
}
