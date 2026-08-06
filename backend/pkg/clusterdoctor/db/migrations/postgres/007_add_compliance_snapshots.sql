-- Postgres variant of 007_add_compliance_snapshots.sql.
-- INTEGER -> BIGINT for unix-second timestamps; SERIAL primary key.
CREATE TABLE IF NOT EXISTS compliance_snapshots (
    id               BIGSERIAL PRIMARY KEY,
    cluster_id       TEXT NOT NULL,
    taken_at         BIGINT NOT NULL,
    score            INTEGER NOT NULL,
    passed           INTEGER NOT NULL,
    failed           INTEGER NOT NULL,
    total            INTEGER NOT NULL,
    failing_controls TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_compliance_snapshots_cluster_time
    ON compliance_snapshots (cluster_id, taken_at DESC);
