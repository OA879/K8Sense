package api

import "testing"

func TestNewlyFailing(t *testing.T) {
	prev := []string{"CIS-5.2.1", "CIS-5.7.4"}
	cur := []string{"CIS-5.2.1", "CIS-5.3.2", "CIS-5.2.6"} // 5.7.4 fixed, 5.3.2 + 5.2.6 new

	got := newlyFailing(prev, cur)
	want := map[string]bool{"CIS-5.3.2": true, "CIS-5.2.6": true}

	if len(got) != len(want) {
		t.Fatalf("newlyFailing = %v, want the two new controls", got)
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected drift control %q", id)
		}
	}
	// No change → no drift.
	if len(newlyFailing(cur, cur)) != 0 {
		t.Error("identical sets should report no drift")
	}
	// A control that was already failing is not 'newly' failing.
	if len(newlyFailing([]string{"CIS-5.2.1"}, []string{"CIS-5.2.1"})) != 0 {
		t.Error("pre-existing failure must not count as drift")
	}
}

func TestFailingControlIDs(t *testing.T) {
	report := complianceReport{Controls: []controlResult{
		{ID: "A", Status: "pass"},
		{ID: "C", Status: "fail"},
		{ID: "B", Status: "fail"},
	}}
	got := failingControlIDs(report)
	if len(got) != 2 || got[0] != "B" || got[1] != "C" {
		t.Errorf("failingControlIDs = %v, want sorted [B C]", got)
	}
}
