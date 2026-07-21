package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// AuditEntry is one recorded action taken through K8sense's Guided Fix — who
// did what, to which resource, and whether it succeeded. Every Guided Fix
// writes exactly one of these, per the "explicit human intent + audit trail"
// requirement for the target (banking/regulated) customers.
type AuditEntry struct {
	ID           string `json:"id"`
	Actor        string `json:"actor"`
	Action       string `json:"action"`
	ClusterID    string `json:"clusterId"`
	Namespace    string `json:"namespace,omitempty"`
	ResourceKind string `json:"resourceKind,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	Payload      string `json:"payload,omitempty"`
	Result       string `json:"result"` // "success" | "failed"
	Error        string `json:"error,omitempty"`
	PerformedAt  int64  `json:"performedAt"`
}

// WriteAudit persists one audit entry. ID and PerformedAt are filled in if the
// caller left them zero.
func WriteAudit(ctx context.Context, database *sql.DB, entry AuditEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}

	_, err := database.ExecContext(ctx, `
		INSERT INTO audit_log (
			id, actor, action, cluster_id, namespace, resource_kind,
			resource_name, payload, result, error, performed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.ID, entry.Actor, entry.Action, entry.ClusterID,
		nullIfEmpty(entry.Namespace), nullIfEmpty(entry.ResourceKind),
		nullIfEmpty(entry.ResourceName), nullIfEmpty(entry.Payload),
		entry.Result, nullIfEmpty(entry.Error), entry.PerformedAt,
	)
	if err != nil {
		return fmt.Errorf("writing audit entry: %w", err)
	}

	return nil
}

// ListAudit returns audit entries for one cluster, most recent first.
func ListAudit(ctx context.Context, database *sql.DB, clusterID string, limit int) ([]AuditEntry, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT id, actor, action, cluster_id,
		       COALESCE(namespace, ''), COALESCE(resource_kind, ''),
		       COALESCE(resource_name, ''), COALESCE(payload, ''),
		       result, COALESCE(error, ''), performed_at
		FROM audit_log
		WHERE cluster_id = ?
		ORDER BY performed_at DESC
		LIMIT ?
	`, clusterID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying audit log: %w", err)
	}
	defer rows.Close()

	var entries []AuditEntry

	for rows.Next() {
		var e AuditEntry

		if err := rows.Scan(
			&e.ID, &e.Actor, &e.Action, &e.ClusterID,
			&e.Namespace, &e.ResourceKind, &e.ResourceName, &e.Payload,
			&e.Result, &e.Error, &e.PerformedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning audit row: %w", err)
		}

		entries = append(entries, e)
	}

	return entries, rows.Err()
}
