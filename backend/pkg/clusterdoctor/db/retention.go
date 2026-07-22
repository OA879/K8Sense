package db

import (
	"context"
	"database/sql"
	"fmt"
)

// PruneScans enforces per-cluster scan retention. Free tier keeps the most
// recent keepPerCluster scans per cluster; Pro keeps everything within a day
// window (handled by the caller passing a large keep count or a separate
// time-based prune). Findings rows are deleted first to satisfy the foreign
// key, then the orphaned scan rows. Called on startup, so a slow prune never
// blocks a user action.
func PruneScans(ctx context.Context, database *sql.DB, keepPerCluster int) (int, error) {
	if keepPerCluster <= 0 {
		return 0, nil
	}

	// Find scan IDs to delete: everything beyond the newest keepPerCluster per
	// cluster, ranked by started_at.
	rows, err := query(ctx, database, `
		SELECT id FROM (
			SELECT id,
			       ROW_NUMBER() OVER (PARTITION BY cluster_id ORDER BY started_at DESC) AS rn
			FROM scans
		) WHERE rn > ?
	`, keepPerCluster)
	if err != nil {
		return 0, fmt.Errorf("selecting scans to prune: %w", err)
	}
	defer rows.Close()

	var ids []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scanning prune id: %w", err)
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning prune tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range ids {
		if _, err := exec(ctx, tx, `DELETE FROM findings WHERE scan_id = ?`, id); err != nil {
			return 0, fmt.Errorf("deleting findings for scan %s: %w", id, err)
		}

		if _, err := exec(ctx, tx, `DELETE FROM scans WHERE id = ?`, id); err != nil {
			return 0, fmt.Errorf("deleting scan %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing prune: %w", err)
	}

	return len(ids), nil
}

// StorageStats reports the SQLite database size and row counts, for the
// Settings storage widget.
type StorageStats struct {
	ScanCount    int   `json:"scanCount"`
	FindingCount int   `json:"findingCount"`
	AuditCount   int   `json:"auditCount"`
	DBSizeBytes  int64 `json:"dbSizeBytes"`
}

// GetStorageStats returns row counts and the on-disk database size.
func GetStorageStats(ctx context.Context, database *sql.DB) (StorageStats, error) {
	var stats StorageStats

	row := queryRow(ctx, database, `
		SELECT
			(SELECT COUNT(*) FROM scans),
			(SELECT COUNT(*) FROM findings),
			(SELECT COUNT(*) FROM audit_log),
			`+storageSizeQuery()+`
	`)
	if err := row.Scan(&stats.ScanCount, &stats.FindingCount, &stats.AuditCount, &stats.DBSizeBytes); err != nil {
		return StorageStats{}, fmt.Errorf("reading storage stats: %w", err)
	}

	return stats, nil
}
