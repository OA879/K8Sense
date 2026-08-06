-- 008: local authentication (users + sessions).
-- Turns K8sense's single, file-based role into real per-user identity for the
-- multi-user web deployment. Passwords are stored only as bcrypt hashes; a
-- session is an opaque random token with an expiry. Auth is opt-in
-- (K8SENSE_AUTH=local) so the single-user desktop app is unaffected.
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer',
    disabled      INTEGER NOT NULL DEFAULT 0,
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions (expires_at);
