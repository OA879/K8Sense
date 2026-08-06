-- Postgres variant of 008_add_users.sql.
-- INTEGER -> BIGINT for unix-second timestamps; disabled kept as an integer
-- boolean to match the codebase's cross-dialect convention (0/1).
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer',
    disabled      INTEGER NOT NULL DEFAULT 0,
    created_at    BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    expires_at BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions (expires_at);
