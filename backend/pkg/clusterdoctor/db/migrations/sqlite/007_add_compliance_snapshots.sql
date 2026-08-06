-- 007: compliance drift monitoring.
-- Each scheduled run records a compliance snapshot per cluster: the overall
-- score plus the set of failing control IDs. Comparing consecutive snapshots
-- is how drift ("a cluster fell out of compliance") is detected and alerted.
CREATE TABLE IF NOT EXISTS compliance_snapshots (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    cluster_id       TEXT NOT NULL,
    taken_at         INTEGER NOT NULL,
    score            INTEGER NOT NULL,
    passed           INTEGER NOT NULL,
    failed           INTEGER NOT NULL,
    total            INTEGER NOT NULL,
    failing_controls TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_compliance_snapshots_cluster_time
    ON compliance_snapshots (cluster_id, taken_at DESC);
