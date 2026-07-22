package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Suppression mutes a finding for a specific resource across scans. It is
// keyed by resource identity — (cluster, rule, namespace, kind, name) — not
// by the per-scan finding UUID, so a suppression keeps applying to the same
// resource even as new scans mint fresh finding IDs.
type Suppression struct {
	ClusterID    string `json:"clusterId"`
	RuleID       string `json:"ruleId"`
	Namespace    string `json:"namespace"`
	ResourceKind string `json:"resourceKind"`
	ResourceName string `json:"resourceName"`
	Reason       string `json:"reason"`
	SuppressedBy string `json:"suppressedBy"`
	SuppressedAt int64  `json:"suppressedAt"`
	Comment      string `json:"comment"`
}

// AddSuppression inserts (or updates) a suppression for a resource. On
// conflict it refreshes the reason, actor and timestamp but leaves any
// existing comment untouched.
func AddSuppression(ctx context.Context, database *sql.DB, s Suppression) error {
	_, err := exec(ctx, database, `
		INSERT INTO suppressions (
			cluster_id, rule_id, namespace, resource_kind, resource_name,
			reason, suppressed_by, suppressed_at, comment
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster_id, rule_id, namespace, resource_kind, resource_name)
		DO UPDATE SET
			reason        = excluded.reason,
			suppressed_by = excluded.suppressed_by,
			suppressed_at = excluded.suppressed_at
	`,
		s.ClusterID, s.RuleID, s.Namespace, s.ResourceKind, s.ResourceName,
		nullIfEmpty(s.Reason), nullIfEmpty(s.SuppressedBy), s.SuppressedAt, nullIfEmpty(s.Comment),
	)
	if err != nil {
		return fmt.Errorf("adding suppression: %w", err)
	}

	return nil
}

// RemoveSuppression deletes the suppression row for a resource by primary key.
func RemoveSuppression(
	ctx context.Context,
	database *sql.DB,
	clusterID, ruleID, namespace, resourceKind, resourceName string,
) error {
	_, err := exec(ctx, database, `
		DELETE FROM suppressions
		WHERE cluster_id = ? AND rule_id = ? AND namespace = ?
		      AND resource_kind = ? AND resource_name = ?
	`, clusterID, ruleID, namespace, resourceKind, resourceName)
	if err != nil {
		return fmt.Errorf("removing suppression: %w", err)
	}

	return nil
}

// SetComment attaches a comment to a resource. If no row exists yet it creates
// a comment-only row (reason NULL, suppressed_at=now) — such rows carry a note
// without muting the finding. If a row already exists (suppressed or not) only
// its comment is updated.
func SetComment(
	ctx context.Context,
	database *sql.DB,
	clusterID, ruleID, namespace, resourceKind, resourceName, comment string,
) error {
	_, err := exec(ctx, database, `
		INSERT INTO suppressions (
			cluster_id, rule_id, namespace, resource_kind, resource_name,
			reason, suppressed_by, suppressed_at, comment
		) VALUES (?, ?, ?, ?, ?, NULL, NULL, ?, ?)
		ON CONFLICT(cluster_id, rule_id, namespace, resource_kind, resource_name)
		DO UPDATE SET comment = excluded.comment
	`,
		clusterID, ruleID, namespace, resourceKind, resourceName,
		time.Now().UTC().Unix(), nullIfEmpty(comment),
	)
	if err != nil {
		return fmt.Errorf("setting comment: %w", err)
	}

	return nil
}

// GetSuppressionKeys returns the set of resource keys that are actively
// suppressed for a cluster. A row counts as suppressed when its reason is
// non-null; comment-only rows (reason NULL) are excluded. Keys are formatted
// as ruleID|namespace|resourceKind|resourceName.
func GetSuppressionKeys(ctx context.Context, database *sql.DB, clusterID string) (map[string]bool, error) {
	rows, err := query(ctx, database, `
		SELECT rule_id, namespace, resource_kind, resource_name
		FROM suppressions
		WHERE cluster_id = ? AND reason IS NOT NULL
	`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("querying suppression keys: %w", err)
	}
	defer rows.Close()

	keys := map[string]bool{}

	for rows.Next() {
		var ruleID, namespace, resourceKind, resourceName string

		if err := rows.Scan(&ruleID, &namespace, &resourceKind, &resourceName); err != nil {
			return nil, fmt.Errorf("scanning suppression key row: %w", err)
		}

		keys[SuppressionKey(ruleID, namespace, resourceKind, resourceName)] = true
	}

	return keys, rows.Err()
}

// GetComments returns a map of resource key -> comment for every row in a
// cluster that carries a non-empty comment. Keys match GetSuppressionKeys.
func GetComments(ctx context.Context, database *sql.DB, clusterID string) (map[string]string, error) {
	rows, err := query(ctx, database, `
		SELECT rule_id, namespace, resource_kind, resource_name, comment
		FROM suppressions
		WHERE cluster_id = ? AND comment IS NOT NULL AND comment <> ''
	`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("querying comments: %w", err)
	}
	defer rows.Close()

	comments := map[string]string{}

	for rows.Next() {
		var ruleID, namespace, resourceKind, resourceName, comment string

		if err := rows.Scan(&ruleID, &namespace, &resourceKind, &resourceName, &comment); err != nil {
			return nil, fmt.Errorf("scanning comment row: %w", err)
		}

		comments[SuppressionKey(ruleID, namespace, resourceKind, resourceName)] = comment
	}

	return comments, rows.Err()
}

// SuppressionKey builds the resource-identity key used to correlate a
// suppression/comment row with a Finding when enriching scan results.
func SuppressionKey(ruleID, namespace, resourceKind, resourceName string) string {
	return ruleID + "|" + namespace + "|" + resourceKind + "|" + resourceName
}
