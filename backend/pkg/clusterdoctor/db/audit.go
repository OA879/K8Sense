package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuditEntry is one recorded action taken on the platform — who did what, to
// which resource, and whether it succeeded. Guided fixes, reverts, and (since
// the audit-everything change) scans, rule and licence changes all write one of
// these, per the "explicit human intent + audit trail" requirement for the
// target (banking/regulated) customers.
//
// Revert fields: a reversible guided fix records RevertAction plus the prior
// state (RevertPayload) needed to undo it. RevertedAt is stamped once the entry
// has been undone. A revert entry itself sets RevertOf to the id of the action
// it reversed. RevertPayload is internal state and is never sent to the client.
type AuditEntry struct {
	ID            string `json:"id"`
	Actor         string `json:"actor"`
	Action        string `json:"action"`
	ClusterID     string `json:"clusterId"`
	Namespace     string `json:"namespace,omitempty"`
	ResourceKind  string `json:"resourceKind,omitempty"`
	ResourceName  string `json:"resourceName,omitempty"`
	Payload       string `json:"payload,omitempty"`
	Result        string `json:"result"` // "success" | "failed"
	Error         string `json:"error,omitempty"`
	PerformedAt   int64  `json:"performedAt"`
	RevertAction  string `json:"revertAction,omitempty"`
	RevertPayload string `json:"-"`
	RevertedAt    *int64 `json:"revertedAt,omitempty"`
	RevertOf      string `json:"revertOf,omitempty"`
}

// WriteAudit persists one audit entry. ID and PerformedAt are filled in if the
// caller left them zero.
func WriteAudit(ctx context.Context, database *sql.DB, entry AuditEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}

	if entry.PerformedAt == 0 {
		entry.PerformedAt = time.Now().UTC().Unix()
	}

	_, err := exec(ctx, database, `
		INSERT INTO audit_log (
			id, actor, action, cluster_id, namespace, resource_kind,
			resource_name, payload, result, error, performed_at,
			revert_action, revert_payload, revert_of
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.ID, entry.Actor, entry.Action, entry.ClusterID,
		nullIfEmpty(entry.Namespace), nullIfEmpty(entry.ResourceKind),
		nullIfEmpty(entry.ResourceName), nullIfEmpty(entry.Payload),
		entry.Result, nullIfEmpty(entry.Error), entry.PerformedAt,
		nullIfEmpty(entry.RevertAction), nullIfEmpty(entry.RevertPayload),
		nullIfEmpty(entry.RevertOf),
	)
	if err != nil {
		return fmt.Errorf("writing audit entry: %w", err)
	}

	return nil
}

const auditSelectColumns = `
	id, actor, action, cluster_id,
	COALESCE(namespace, ''), COALESCE(resource_kind, ''),
	COALESCE(resource_name, ''), COALESCE(payload, ''),
	result, COALESCE(error, ''), performed_at,
	COALESCE(revert_action, ''), COALESCE(revert_payload, ''),
	reverted_at, COALESCE(revert_of, '')
`

func scanAudit(row interface{ Scan(...any) error }) (AuditEntry, error) {
	var (
		e          AuditEntry
		revertedAt sql.NullInt64
	)

	if err := row.Scan(
		&e.ID, &e.Actor, &e.Action, &e.ClusterID,
		&e.Namespace, &e.ResourceKind, &e.ResourceName, &e.Payload,
		&e.Result, &e.Error, &e.PerformedAt,
		&e.RevertAction, &e.RevertPayload, &revertedAt, &e.RevertOf,
	); err != nil {
		return AuditEntry{}, err
	}

	if revertedAt.Valid {
		e.RevertedAt = &revertedAt.Int64
	}

	return e, nil
}

// ListAudit returns audit entries for one cluster, most recent first.
func ListAudit(ctx context.Context, database *sql.DB, clusterID string, limit int) ([]AuditEntry, error) {
	rows, err := query(ctx, database, `
		SELECT `+auditSelectColumns+`
		FROM audit_log
		WHERE cluster_id = ?
		ORDER BY performed_at DESC
		LIMIT ?
	`, clusterID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying audit log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []AuditEntry

	for rows.Next() {
		e, err := scanAudit(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning audit row: %w", err)
		}

		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// GetAudit fetches a single audit entry by id, or sql.ErrNoRows if absent.
func GetAudit(ctx context.Context, database *sql.DB, id string) (AuditEntry, error) {
	row := queryRow(ctx, database, `
		SELECT `+auditSelectColumns+`
		FROM audit_log
		WHERE id = ?
	`, id)

	e, err := scanAudit(row)
	if err != nil {
		return AuditEntry{}, err
	}

	return e, nil
}

// MarkReverted stamps reverted_at on an entry so it can't be undone twice and
// the UI can render it as already reverted.
func MarkReverted(ctx context.Context, database *sql.DB, id string, at int64) error {
	if _, err := exec(ctx, database,
		`UPDATE audit_log SET reverted_at = ? WHERE id = ?`, at, id); err != nil {
		return fmt.Errorf("marking audit entry reverted: %w", err)
	}

	return nil
}
