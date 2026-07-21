package db

import (
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
// STATUS: this is groundwork, deliberately not yet wired into Open. SQLite is
// currently the only supported backend, and web-mode deployments must run a
// single replica (see deploy/k8sense-web.yaml). Completing the Postgres path
// additionally requires porting the migration set — 005 in particular rebuilds
// a table, which is the SQLite idiom for dropping a constraint and is not how
// Postgres would express it. Shipping a partially-verified second persistence
// path for an audit log would be worse than shipping none, so the remaining
// work is tracked rather than half-done. The rebinding below is fully tested
// so that port is a smaller, safer change when it happens.
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
