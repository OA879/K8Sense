package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CustomRule is a user-imported rule stored in the custom_rules table. The
// full rule definition lives in YAMLContent; ID/Name are denormalized for
// listing.
type CustomRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	YAML       string `json:"yaml"`
	Enabled    bool   `json:"enabled"`
	ImportedAt int64  `json:"importedAt"`
}

// AddCustomRule inserts or replaces a custom rule.
func AddCustomRule(ctx context.Context, database *sql.DB, id, name, yamlContent string) error {
	_, err := exec(ctx, database, `
		INSERT INTO custom_rules (id, name, yaml_content, enabled, imported_at)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, yaml_content = excluded.yaml_content
	`, id, name, yamlContent, time.Now().UTC().Unix())
	if err != nil {
		return fmt.Errorf("adding custom rule: %w", err)
	}

	return nil
}

// ListCustomRules returns all imported custom rules.
func ListCustomRules(ctx context.Context, database *sql.DB) ([]CustomRule, error) {
	rows, err := query(ctx, database, `
		SELECT id, name, yaml_content, enabled, imported_at
		FROM custom_rules ORDER BY imported_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing custom rules: %w", err)
	}
	defer rows.Close()

	var out []CustomRule

	for rows.Next() {
		var c CustomRule

		var enabled int

		if err := rows.Scan(&c.ID, &c.Name, &c.YAML, &enabled, &c.ImportedAt); err != nil {
			return nil, fmt.Errorf("scanning custom rule: %w", err)
		}

		c.Enabled = enabled == 1
		out = append(out, c)
	}

	return out, rows.Err()
}

// DeleteCustomRule removes a custom rule by id.
func DeleteCustomRule(ctx context.Context, database *sql.DB, id string) error {
	_, err := exec(ctx, database, `DELETE FROM custom_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting custom rule: %w", err)
	}

	return nil
}
