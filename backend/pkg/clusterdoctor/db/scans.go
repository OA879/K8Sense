package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kubernetes-sigs/headlamp/backend/pkg/clusterdoctor"
)

// ScanSummary is one row of scan history, as shown on the HistoryPage.
type ScanSummary struct {
	ID            string `json:"id"`
	ClusterID     string `json:"clusterId"`
	StartedAt     int64  `json:"startedAt"`
	CompletedAt   *int64 `json:"completedAt,omitempty"`
	Status        string `json:"status"`
	TotalFindings int    `json:"totalFindings"`
	CriticalCount int    `json:"criticalCount"`
	WarningCount  int    `json:"warningCount"`
	InfoCount     int    `json:"infoCount"`
	SkippedChecks int    `json:"skippedChecks"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
}

// SaveScan persists a completed clusterdoctor.ScanResult and all of its
// findings in a single transaction.
func SaveScan(ctx context.Context, database *sql.DB, result *clusterdoctor.ScanResult) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	var critical, warning, info int

	for _, f := range result.Findings {
		switch f.Severity {
		case clusterdoctor.SeverityCritical:
			critical++
		case clusterdoctor.SeverityWarning:
			warning++
		case clusterdoctor.SeverityInfo:
			info++
		}
	}

	var completedAt any
	if !result.CompletedAt.IsZero() {
		completedAt = result.CompletedAt.Unix()
	}

	_, err = exec(ctx, tx, `
		INSERT INTO scans (
			id, cluster_id, started_at, completed_at, status,
			total_findings, critical_count, warning_count, info_count,
			skipped_checks, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		result.ID, result.Cluster, result.StartedAt.Unix(), completedAt, string(result.Status),
		len(result.Findings), critical, warning, info,
		result.SkippedChecks, nullIfEmpty(result.ErrorMessage),
	)
	if err != nil {
		return fmt.Errorf("inserting scan: %w", err)
	}

	// Rebind for the active dialect: a prepared statement's placeholders are
	// not routed through the exec helper, so they must be converted here too.
	stmt, err := tx.PrepareContext(ctx, rebind(`
		INSERT INTO findings (
			id, scan_id, rule_id, rule_name, severity, category,
			namespace, resource_kind, resource_name, description,
			remediation, raw_object, detected_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`))
	if err != nil {
		return fmt.Errorf("preparing findings insert: %w", err)
	}
	defer stmt.Close()

	for _, f := range result.Findings {
		_, err := stmt.ExecContext(ctx,
			f.ID, f.ScanID, f.RuleID, f.RuleName, f.Severity, f.Category,
			nullIfEmpty(f.Namespace), f.ResourceKind, f.ResourceName, f.Description,
			f.Remediation, nullIfEmpty(f.RawObject), f.DetectedAt.Unix(),
		)
		if err != nil {
			return fmt.Errorf("inserting finding %s: %w", f.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing scan: %w", err)
	}

	return nil
}

// GetFindings returns every finding recorded for scanID, most severe first.
func GetFindings(ctx context.Context, database *sql.DB, scanID string) ([]clusterdoctor.Finding, error) {
	rows, err := query(ctx, database, `
		SELECT id, scan_id, rule_id, rule_name, severity, category,
		       COALESCE(namespace, ''), resource_kind, resource_name, description,
		       remediation, COALESCE(raw_object, ''), detected_at
		FROM findings
		WHERE scan_id = ?
		ORDER BY
			CASE severity WHEN 'CRITICAL' THEN 0 WHEN 'WARNING' THEN 1 ELSE 2 END,
			category, resource_name
	`, scanID)
	if err != nil {
		return nil, fmt.Errorf("querying findings: %w", err)
	}
	defer rows.Close()

	var findings []clusterdoctor.Finding

	for rows.Next() {
		var f clusterdoctor.Finding

		var detectedAt int64

		if err := rows.Scan(
			&f.ID, &f.ScanID, &f.RuleID, &f.RuleName, &f.Severity, &f.Category,
			&f.Namespace, &f.ResourceKind, &f.ResourceName, &f.Description,
			&f.Remediation, &f.RawObject, &detectedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning finding row: %w", err)
		}

		f.DetectedAt = unixToTime(detectedAt)
		findings = append(findings, f)
	}

	return findings, rows.Err()
}

// GetScan returns the summary row for a single scan, or sql.ErrNoRows if no
// such scan exists.
func GetScan(ctx context.Context, database *sql.DB, scanID string) (ScanSummary, error) {
	var s ScanSummary

	var completedAt sql.NullInt64

	err := queryRow(ctx, database, `
		SELECT id, cluster_id, started_at, completed_at, status,
		       total_findings, critical_count, warning_count, info_count,
		       skipped_checks, COALESCE(error_message, '')
		FROM scans
		WHERE id = ?
	`, scanID).Scan(
		&s.ID, &s.ClusterID, &s.StartedAt, &completedAt, &s.Status,
		&s.TotalFindings, &s.CriticalCount, &s.WarningCount, &s.InfoCount,
		&s.SkippedChecks, &s.ErrorMessage,
	)
	if err != nil {
		return ScanSummary{}, err
	}

	if completedAt.Valid {
		s.CompletedAt = &completedAt.Int64
	}

	return s, nil
}

// ListScans returns scan history for one cluster, most recent first.
func ListScans(ctx context.Context, database *sql.DB, clusterID string, limit int) ([]ScanSummary, error) {
	rows, err := query(ctx, database, `
		SELECT id, cluster_id, started_at, completed_at, status,
		       total_findings, critical_count, warning_count, info_count,
		       skipped_checks, COALESCE(error_message, '')
		FROM scans
		WHERE cluster_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`, clusterID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying scan history: %w", err)
	}
	defer rows.Close()

	var scans []ScanSummary

	for rows.Next() {
		var s ScanSummary

		var completedAt sql.NullInt64

		if err := rows.Scan(
			&s.ID, &s.ClusterID, &s.StartedAt, &completedAt, &s.Status,
			&s.TotalFindings, &s.CriticalCount, &s.WarningCount, &s.InfoCount,
			&s.SkippedChecks, &s.ErrorMessage,
		); err != nil {
			return nil, fmt.Errorf("scanning scan row: %w", err)
		}

		if completedAt.Valid {
			s.CompletedAt = &completedAt.Int64
		}

		scans = append(scans, s)
	}

	return scans, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}

	return s
}
