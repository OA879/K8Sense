package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ComplianceSnapshot is one point-in-time compliance result for a cluster,
// recorded on each scheduled run. Comparing consecutive snapshots is how drift
// is detected — a lower score or a newly-failing control means the cluster
// slipped out of compliance since last time.
type ComplianceSnapshot struct {
	ClusterID       string   `json:"clusterId"`
	TakenAt         int64    `json:"takenAt"`
	Score           int      `json:"score"`
	Passed          int      `json:"passed"`
	Failed          int      `json:"failed"`
	Total           int      `json:"total"`
	FailingControls []string `json:"failingControls"`
}

// SaveComplianceSnapshot records a snapshot.
func SaveComplianceSnapshot(ctx context.Context, database *sql.DB, snap ComplianceSnapshot) error {
	controls, err := json.Marshal(snap.FailingControls)
	if err != nil {
		return fmt.Errorf("encoding failing controls: %w", err)
	}

	_, err = exec(ctx, database, `
		INSERT INTO compliance_snapshots
			(cluster_id, taken_at, score, passed, failed, total, failing_controls)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, snap.ClusterID, snap.TakenAt, snap.Score, snap.Passed, snap.Failed, snap.Total, string(controls))
	if err != nil {
		return fmt.Errorf("saving compliance snapshot: %w", err)
	}

	return nil
}

// LatestComplianceSnapshot returns the most recent snapshot for a cluster.
// ok is false when the cluster has no history yet.
func LatestComplianceSnapshot(ctx context.Context, database *sql.DB, clusterID string) (ComplianceSnapshot, bool, error) {
	snap := ComplianceSnapshot{ClusterID: clusterID}

	var controls string

	err := queryRow(ctx, database, `
		SELECT taken_at, score, passed, failed, total, failing_controls
		FROM compliance_snapshots
		WHERE cluster_id = ?
		ORDER BY taken_at DESC, id DESC
		LIMIT 1
	`, clusterID).Scan(&snap.TakenAt, &snap.Score, &snap.Passed, &snap.Failed, &snap.Total, &controls)

	if errors.Is(err, sql.ErrNoRows) {
		return snap, false, nil
	}

	if err != nil {
		return snap, false, fmt.Errorf("reading latest compliance snapshot: %w", err)
	}

	_ = json.Unmarshal([]byte(controls), &snap.FailingControls)

	return snap, true, nil
}

// ListComplianceSnapshots returns a cluster's snapshots, oldest first, capped at
// limit — used to draw the compliance trend.
func ListComplianceSnapshots(ctx context.Context, database *sql.DB, clusterID string, limit int) ([]ComplianceSnapshot, error) {
	if limit <= 0 {
		limit = 50
	}

	// Pull the newest `limit` rows, then reverse to oldest-first for charting.
	rows, err := query(ctx, database, `
		SELECT taken_at, score, passed, failed, total, failing_controls
		FROM compliance_snapshots
		WHERE cluster_id = ?
		ORDER BY taken_at DESC, id DESC
		LIMIT ?
	`, clusterID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing compliance snapshots: %w", err)
	}
	defer rows.Close()

	var snaps []ComplianceSnapshot

	for rows.Next() {
		snap := ComplianceSnapshot{ClusterID: clusterID}

		var controls string

		if err := rows.Scan(&snap.TakenAt, &snap.Score, &snap.Passed, &snap.Failed, &snap.Total, &controls); err != nil {
			return nil, fmt.Errorf("scanning snapshot row: %w", err)
		}

		_ = json.Unmarshal([]byte(controls), &snap.FailingControls)
		snaps = append(snaps, snap)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to oldest-first.
	for i, j := 0, len(snaps)-1; i < j; i, j = i+1, j-1 {
		snaps[i], snaps[j] = snaps[j], snaps[i]
	}

	return snaps, nil
}
