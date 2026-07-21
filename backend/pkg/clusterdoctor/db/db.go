// Package db is Cluster Doctor's SQLite persistence layer: scan history,
// findings, audit log, and per-cluster preferences. It never needs external
// infrastructure — the whole thing is one file on disk, opened with the
// pure-Go modernc.org/sqlite driver so K8sense stays a single static binary.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open creates (if needed) the directory containing path, opens the SQLite
// database there in WAL mode, and applies any migrations that haven't run
// yet. path should come from DefaultPath() in normal operation; tests pass
// their own temp-file path.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:mnd
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// WAL mode lets the scanner write findings while the API concurrently
	// reads history without lock contention.
	if _, err := database.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}

	if _, err := database.Exec(`PRAGMA foreign_keys=ON;`); err != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	if err := migrate(database); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return database, nil
}

// DefaultPath returns {OS app data dir}/k8sense/k8sense.db, matching:
//   - macOS:   ~/Library/Application Support/k8sense/k8sense.db
//   - Linux:   ~/.local/share/k8sense/k8sense.db
//   - Windows: %APPDATA%\k8sense\k8sense.db
func DefaultPath() (string, error) {
	// K8SENSE_DB_PATH lets a containerised (web-mode) deployment point the
	// database at a mounted volume. Without it the database would land on the
	// pod's ephemeral filesystem and every scan history and audit-log entry
	// would be destroyed on restart — unacceptable for a compliance record.
	if override := os.Getenv("K8SENSE_DB_PATH"); override != "" {
		return override, nil
	}

	base, err := appDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "k8sense", "k8sense.db"), nil
}

func appDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support"), nil
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return appData, nil
		}

		return filepath.Join(home, "AppData", "Roaming"), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return xdg, nil
		}

		return filepath.Join(home, ".local", "share"), nil
	}
}

// migrate applies every migrations/NNN_*.sql file whose number is greater
// than the highest already-recorded in schema_version, in numeric order.
// Per K8SENSE_CONTEXT.md: never edit an applied migration file — add a new
// numbered one instead.
func migrate(database *sql.DB) error {
	if _, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version    INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		);
	`); err != nil {
		return err
	}

	var current int

	row := database.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version;`)
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("reading schema_version: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading embedded migrations: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		var version int

		if _, err := fmt.Sscanf(entry.Name(), "%d_", &version); err != nil {
			continue // not a numbered migration file
		}

		if version <= current {
			continue
		}

		sqlBytes, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", entry.Name(), err)
		}

		tx, err := database.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration transaction: %w", err)
		}

		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			_ = tx.Rollback()

			return fmt.Errorf("applying migration %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec(
			`INSERT INTO schema_version (version, applied_at) VALUES (?, ?);`,
			version, time.Now().UTC().Unix(),
		); err != nil {
			_ = tx.Rollback()

			return fmt.Errorf("recording migration %s: %w", entry.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}
