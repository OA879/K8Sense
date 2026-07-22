package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

func TestOpenMigrateSaveAndReadScan(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "k8sense.db")

	database, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	result := &clusterdoctor.ScanResult{
		ID:          "scan-1",
		Cluster:     "kind-k8sense-dev",
		StartedAt:   time.Now().Add(-time.Minute).UTC(),
		CompletedAt: time.Now().UTC(),
		Status:      clusterdoctor.ScanCompleted,
		Findings: []clusterdoctor.Finding{
			{
				ID:           "finding-1",
				ScanID:       "scan-1",
				RuleID:       "NODE-001",
				RuleName:     "Node Not Ready",
				Severity:     clusterdoctor.SeverityCritical,
				Category:     "nodes",
				ResourceKind: "Node",
				ResourceName: "worker-1",
				Description:  "desc",
				Remediation:  "fix it",
				DetectedAt:   time.Now().UTC(),
			},
		},
	}

	ctx := context.Background()

	if err := SaveScan(ctx, database, result); err != nil {
		t.Fatalf("SaveScan: %v", err)
	}

	findings, err := GetFindings(ctx, database, "scan-1")
	if err != nil {
		t.Fatalf("GetFindings: %v", err)
	}

	if len(findings) != 1 || findings[0].RuleID != "NODE-001" {
		t.Fatalf("unexpected findings: %+v", findings)
	}

	scans, err := ListScans(ctx, database, "kind-k8sense-dev", 10)
	if err != nil {
		t.Fatalf("ListScans: %v", err)
	}

	if len(scans) != 1 || scans[0].CriticalCount != 1 {
		t.Fatalf("unexpected scan history: %+v", scans)
	}
}
