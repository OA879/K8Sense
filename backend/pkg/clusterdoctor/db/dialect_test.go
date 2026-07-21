package db

import "testing"

// withDialect temporarily switches the active dialect for one test.
func withDialect(t *testing.T, d Dialect) {
	t.Helper()

	previous := CurrentDialect()
	setDialect(d)
	t.Cleanup(func() { setDialect(previous) })
}

func TestRebindLeavesSQLiteQueriesAlone(t *testing.T) {
	withDialect(t, DialectSQLite)

	q := `SELECT * FROM scans WHERE cluster_id = ? AND status = ?`
	if got := rebind(q); got != q {
		t.Errorf("SQLite query was rewritten:\n got %q\nwant %q", got, q)
	}
}

func TestRebindNumbersPostgresPlaceholders(t *testing.T) {
	withDialect(t, DialectPostgres)

	cases := []struct{ in, want string }{
		{
			`SELECT * FROM scans WHERE cluster_id = ? AND status = ?`,
			`SELECT * FROM scans WHERE cluster_id = $1 AND status = $2`,
		},
		{
			`INSERT INTO audit_log (id, actor, action) VALUES (?, ?, ?)`,
			`INSERT INTO audit_log (id, actor, action) VALUES ($1, $2, $3)`,
		},
		{`SELECT 1`, `SELECT 1`},
	}

	for _, tc := range cases {
		if got := rebind(tc.in); got != tc.want {
			t.Errorf("rebind(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

// TestRebindNumbersBeyondNine catches an off-by-one in multi-digit numbering —
// SaveScan inserts far more than nine columns, so this is a real case.
func TestRebindNumbersBeyondNine(t *testing.T) {
	withDialect(t, DialectPostgres)

	in := `INSERT INTO findings VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	want := `INSERT INTO findings VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	if got := rebind(in); got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

// TestRebindIgnoresQuestionMarksInLiterals is the correctness case: rewriting a
// '?' that is part of a string value would silently corrupt the query.
func TestRebindIgnoresQuestionMarksInLiterals(t *testing.T) {
	withDialect(t, DialectPostgres)

	cases := []struct{ in, want string }{
		{
			`SELECT * FROM findings WHERE remediation = 'what?' AND rule_id = ?`,
			`SELECT * FROM findings WHERE remediation = 'what?' AND rule_id = $1`,
		},
		{
			// An escaped quote inside a literal must not end the literal.
			`SELECT * FROM t WHERE a = 'it''s ? here' AND b = ?`,
			`SELECT * FROM t WHERE a = 'it''s ? here' AND b = $1`,
		},
	}

	for _, tc := range cases {
		if got := rebind(tc.in); got != tc.want {
			t.Errorf("rebind(%q)\n got %q\nwant %q", tc.in, got, tc.want)
		}
	}
}

func TestStorageSizeQueryPerDialect(t *testing.T) {
	withDialect(t, DialectSQLite)

	if got := storageSizeQuery(); got == "" || !contains(got, "page_count") {
		t.Errorf("SQLite storage query = %q", got)
	}

	withDialect(t, DialectPostgres)

	if got := storageSizeQuery(); !contains(got, "pg_database_size") {
		t.Errorf("Postgres storage query = %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}

		return false
	})()
}

func TestItoa(t *testing.T) {
	cases := map[int]string{1: "1", 9: "9", 10: "10", 12: "12", 105: "105"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}
