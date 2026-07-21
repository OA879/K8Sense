package db

import (
	"context"
	"database/sql"
	"fmt"
)

// SetRuleOverride records whether a rule is enabled for a specific cluster.
// It UPSERTs into rule_overrides so toggling the same rule repeatedly just
// updates the existing row rather than piling up duplicates. A rule with no
// row is treated as enabled (the default), so only deviations from that
// default are stored.
func SetRuleOverride(ctx context.Context, database *sql.DB, clusterID, ruleID string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	_, err := database.ExecContext(ctx, `
		INSERT INTO rule_overrides (cluster_id, rule_id, enabled)
		VALUES (?, ?, ?)
		ON CONFLICT(cluster_id, rule_id) DO UPDATE SET enabled = excluded.enabled
	`, clusterID, ruleID, enabledInt)
	if err != nil {
		return fmt.Errorf("upserting rule override: %w", err)
	}

	return nil
}

// SetRuleSeverity records a per-cluster severity override for a rule. An empty
// severity clears the override (reverting to the rule's default). The row's
// enabled state defaults to 1 (enabled) when the row is first created here.
func SetRuleSeverity(ctx context.Context, database *sql.DB, clusterID, ruleID, severity string) error {
	var sev any
	if severity != "" {
		sev = severity
	}

	_, err := database.ExecContext(ctx, `
		INSERT INTO rule_overrides (cluster_id, rule_id, enabled, severity_override)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(cluster_id, rule_id) DO UPDATE SET severity_override = excluded.severity_override
	`, clusterID, ruleID, sev)
	if err != nil {
		return fmt.Errorf("upserting rule severity override: %w", err)
	}

	return nil
}

// GetSeverityOverrides returns ruleID -> overridden severity for clusterID
// (only rows where a severity override is set).
func GetSeverityOverrides(ctx context.Context, database *sql.DB, clusterID string) (map[string]string, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT rule_id, severity_override
		FROM rule_overrides
		WHERE cluster_id = ? AND severity_override IS NOT NULL AND severity_override != ''
	`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("querying severity overrides: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}

	for rows.Next() {
		var ruleID, sev string
		if err := rows.Scan(&ruleID, &sev); err != nil {
			return nil, fmt.Errorf("scanning severity override: %w", err)
		}

		out[ruleID] = sev
	}

	return out, rows.Err()
}

// GetDisabledRuleIDs returns the set of rule IDs that have been explicitly
// disabled (enabled = 0) for clusterID. Rules absent from the map are enabled,
// which lets callers treat the default as "on" without reading every rule row.
func GetDisabledRuleIDs(ctx context.Context, database *sql.DB, clusterID string) (map[string]bool, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT rule_id
		FROM rule_overrides
		WHERE cluster_id = ? AND enabled = 0
	`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("querying disabled rules: %w", err)
	}
	defer rows.Close()

	disabled := map[string]bool{}

	for rows.Next() {
		var ruleID string
		if err := rows.Scan(&ruleID); err != nil {
			return nil, fmt.Errorf("scanning disabled rule row: %w", err)
		}

		disabled[ruleID] = true
	}

	return disabled, rows.Err()
}
