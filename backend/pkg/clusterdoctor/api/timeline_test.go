package api

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
	cddb "github.com/OA879/K8Sense/backend/pkg/clusterdoctor/db"
)

func TestMergeTimeline_SortsFiltersAndConverts(t *testing.T) {
	now := time.Now()
	since := now.Add(-30 * time.Minute)

	audits := []cddb.AuditEntry{
		{Action: "scale_deployment", Result: "success", Actor: "alice",
			Namespace: "shop", ResourceName: "orders", PerformedAt: now.Add(-10 * time.Minute).Unix()},
		{Action: "delete_pod", Result: "failed", Actor: "bob",
			Namespace: "shop", ResourceName: "orders", PerformedAt: now.Add(-2 * time.Hour).Unix()}, // too old
	}
	events := []corev1.Event{
		{Type: corev1.EventTypeWarning, Reason: "OOMKilling", Message: "killed",
			Count: 3, LastTimestamp: metav1.Time{Time: now.Add(-5 * time.Minute)},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "shop", Name: "orders-abc"}},
		{Type: corev1.EventTypeNormal, Reason: "Scheduled", Message: "ok",
			LastTimestamp: metav1.Time{Time: now.Add(-1 * time.Minute)},
			InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "other", Name: "web-1"}},
	}
	findings := []clusterdoctor.Finding{
		{RuleName: "CrashLoopBackOff", Severity: "CRITICAL", Namespace: "shop",
			ResourceName: "orders-abc", DetectedAt: now.Add(-3 * time.Minute)},
	}

	// No filters except the 30-min window: the 2h-old audit is dropped.
	all := mergeTimeline(audits, events, findings, timelineOpts{since: since})
	if len(all) != 4 {
		t.Fatalf("got %d entries, want 4 (only the 2h-old audit filtered by window)", len(all))
	}
	// Newest first: -1m Scheduled event.
	if all[0].Title != "Scheduled" {
		t.Errorf("first (newest) = %q, want Scheduled", all[0].Title)
	}
	// Conversions.
	var sawScale, sawOOM, sawFinding bool
	for _, e := range all {
		switch e.Type {
		case "action":
			sawScale = e.Title == "Scaled deployment" && e.Level == "info" && e.Actor == "alice"
		case "event":
			if e.Title == "OOMKilling (x3)" && e.Level == "warning" {
				sawOOM = true
			}
		case "finding":
			sawFinding = e.Level == "critical" && e.Title == "CrashLoopBackOff"
		}
	}
	if !sawScale || !sawOOM || !sawFinding {
		t.Errorf("conversions off: scale=%v oom=%v finding=%v", sawScale, sawOOM, sawFinding)
	}

	// Namespace filter keeps only shop -> drops the "other" Scheduled event.
	shop := mergeTimeline(audits, events, findings, timelineOpts{since: since, namespace: "shop"})
	for _, e := range shop {
		if e.Namespace != "shop" {
			t.Errorf("namespace filter leaked %s", e.Namespace)
		}
	}

	// Resource substring filter.
	orders := mergeTimeline(audits, events, findings, timelineOpts{since: since, resource: "orders"})
	if len(orders) != 3 {
		t.Errorf("resource=orders got %d, want 3 (all touch 'orders')", len(orders))
	}
}

func TestHumanizeAction_Revert(t *testing.T) {
	if got := humanizeAction("revert_scale_deployment"); got != "Reverted: Scaled deployment" {
		t.Fatalf("got %q", got)
	}
}
