// Package db is Cluster Doctor's persistence layer: scan history, findings,
// audit log, and per-cluster preferences.
//
// It supports two backends. SQLite is the default — one file on disk, opened
// with the pure-Go modernc.org/sqlite driver so K8sense stays a single static
// binary with no external infrastructure, which is what a desktop install and
// a single-pod web deployment want. Postgres is available for hosted
// deployments that need more than one replica: SQLite is a single-writer store
// and cannot serve that safely. The backend is chosen by the connection
// string (see Open).
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"  // registers the "postgres" database/sql driver
	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

//go:embed migrations/sqlite/*.sql migrations/postgres/*.sql
var migrationsFS embed.FS

// Open connects to the persistence backend named by dsn and applies any
// pending migrations. A dsn beginning with "postgres://" or "postgresql://"
// selects the Postgres backend; anything else is treated as a SQLite file path
// (the desktop default, e.g. from DefaultPath()). Tests pass their own dsn.
func Open(dsn string) (*sql.DB, error) {
	if IsPostgresDSN(dsn) {
		return openPostgres(dsn)
	}

	return openSQLite(dsn)
}

// IsPostgresDSN reports whether dsn selects the Postgres backend.
func IsPostgresDSN(dsn string) bool {
	return strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
}

func openSQLite(path string) (*sql.DB, error) {
	setDialect(DialectSQLite)

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

func openPostgres(dsn string) (*sql.DB, error) {
	setDialect(DialectPostgres)

	database, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres: %w", err)
	}

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
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

	// Migrations are dialect-specific (the two dirs hold the same numbered
	// sequence with per-backend type differences), selected by the active
	// dialect so the schema_version numbers stay aligned across backends.
	migrationsDir := "migrations/sqlite"
	if CurrentDialect() == DialectPostgres {
		migrationsDir = "migrations/postgres"
	}

	entries, err := migrationsFS.ReadDir(migrationsDir)
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

		sqlBytes, err := migrationsFS.ReadFile(migrationsDir + "/" + entry.Name())
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
			rebind(`INSERT INTO schema_version (version, applied_at) VALUES (?, ?);`),
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
