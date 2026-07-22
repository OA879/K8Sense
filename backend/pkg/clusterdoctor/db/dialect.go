package db

import (
	"context"
	"database/sql"
	"strings"
	"sync"
)

// Dialect identifies which SQL backend is in use. K8sense supports two:
//
//   - SQLite  — the desktop default. One file, no infrastructure, single writer.
//   - Postgres — for hosted (web-mode) deployments that need more than one
//     replica. SQLite cannot serve that safely: it is a single-writer store, so
//     two pods sharing a volume will corrupt it.
//
// Queries throughout this package are written once, with `?` placeholders, and
// rebound per dialect. That keeps a single source of truth for every statement
// rather than two divergent copies that can drift apart — which for an audit
// log would be a correctness problem, not just a maintenance one.
//
// Both backends are wired into Open (chosen by the connection string) and
// exercised end-to-end: the full DB layer runs against a real Postgres in
// db.TestPostgresBackendEndToEnd, and the existing SQLite suite guards the
// default path. Migrations are dialect-specific (migrations/{sqlite,postgres})
// so each backend gets correct column types — notably BIGINT for the
// unix-second timestamp columns, which would overflow a 32-bit Postgres
// INTEGER in 2038.
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

var (
	activeDialectMu sync.RWMutex
	activeDialect   = DialectSQLite
)

// setDialect records which backend Open connected to.
func setDialect(d Dialect) {
	activeDialectMu.Lock()
	defer activeDialectMu.Unlock()
	activeDialect = d
}

// CurrentDialect reports the active backend.
func CurrentDialect() Dialect {
	activeDialectMu.RLock()
	defer activeDialectMu.RUnlock()

	return activeDialect
}

// rebind converts the `?` placeholders every query in this package is written
// with into the form the active driver expects. SQLite takes `?` as-is;
// Postgres needs positional `$1, $2, …`.
//
// Question marks inside string literals must not be rewritten, so the scan
// tracks whether it is inside a quoted literal.
func rebind(query string) string {
	if CurrentDialect() != DialectPostgres {
		return query
	}

	var (
		out       strings.Builder
		arg       int
		inLiteral bool
	)

	out.Grow(len(query) + 8) //nolint:mnd // small headroom for $N expansion

	for i := 0; i < len(query); i++ {
		c := query[i]

		if c == '\'' {
			// '' inside a literal is an escaped quote, not a terminator.
			if inLiteral && i+1 < len(query) && query[i+1] == '\'' {
				out.WriteByte(c)
				out.WriteByte(query[i+1])
				i++

				continue
			}

			inLiteral = !inLiteral

			out.WriteByte(c)

			continue
		}

		if c == '?' && !inLiteral {
			arg++

			out.WriteByte('$')
			out.WriteString(itoa(arg))

			continue
		}

		out.WriteByte(c)
	}

	return out.String()
}

// itoa avoids pulling strconv into a hot path for small positive integers.
func itoa(n int) string {
	if n < 10 { //nolint:mnd // single digit fast path
		return string(rune('0' + n))
	}

	var digits []byte

	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	return string(digits)
}

// storageSizeQuery returns the dialect's way of measuring on-disk size.
func storageSizeQuery() string {
	if CurrentDialect() == DialectPostgres {
		return `(SELECT pg_database_size(current_database()))`
	}

	return `(SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size())`
}

// querier is satisfied by both *sql.DB and *sql.Tx, so the rebinding helpers
// below work inside and outside a transaction without duplicated code.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// exec / query / queryRow are the single choke point every statement in this
// package flows through. They rebind `?` placeholders to the active dialect's
// form, so query strings are written once and correct on both backends. For
// SQLite rebind is a no-op, so these are transparent on the existing path.
func exec(ctx context.Context, q querier, query string, args ...any) (sql.Result, error) {
	return q.ExecContext(ctx, rebind(query), args...)
}

func query(ctx context.Context, q querier, query string, args ...any) (*sql.Rows, error) {
	return q.QueryContext(ctx, rebind(query), args...)
}

func queryRow(ctx context.Context, q querier, query string, args ...any) *sql.Row {
	return q.QueryRowContext(ctx, rebind(query), args...)
}
