package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestComplianceSnapshots_RoundTripAndLatest(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()

	if _, ok, _ := LatestComplianceSnapshot(ctx, database, "c1"); ok {
		t.Error("no snapshot should exist yet")
	}

	s1 := ComplianceSnapshot{ClusterID: "c1", TakenAt: 100, Score: 80, Passed: 8, Failed: 2, Total: 10, FailingControls: []string{"X", "Y"}}
	s2 := ComplianceSnapshot{ClusterID: "c1", TakenAt: 200, Score: 70, Passed: 7, Failed: 3, Total: 10, FailingControls: []string{"X", "Y", "Z"}}
	for _, s := range []ComplianceSnapshot{s1, s2} {
		if err := SaveComplianceSnapshot(ctx, database, s); err != nil {
			t.Fatal(err)
		}
	}

	latest, ok, err := LatestComplianceSnapshot(ctx, database, "c1")
	if err != nil || !ok {
		t.Fatalf("expected a latest snapshot: %v ok=%v", err, ok)
	}
	if latest.TakenAt != 200 || latest.Score != 70 || len(latest.FailingControls) != 3 {
		t.Errorf("latest snapshot wrong: %+v", latest)
	}

	// History is oldest-first.
	hist, err := ListComplianceSnapshots(ctx, database, "c1", 50)
	if err != nil || len(hist) != 2 {
		t.Fatalf("history len = %d, err=%v", len(hist), err)
	}
	if hist[0].TakenAt != 100 || hist[1].TakenAt != 200 {
		t.Errorf("history not oldest-first: %d then %d", hist[0].TakenAt, hist[1].TakenAt)
	}
}
