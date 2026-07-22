-- Postgres variant of 005_fix_scan_schedules_fk.sql. Generated from the SQLite schema;
-- INTEGER -> BIGINT so unix-second timestamps do not overflow int32 in 2038.
-- scan_schedules was defined in 001 with `cluster_id REFERENCES clusters(id)`,
-- but nothing ever writes to the clusters table — every other per-cluster
-- table (scans, rule_overrides, suppressions, notification_config) keys on the
-- cluster *name* with no foreign key. The FK therefore made it impossible to
-- save a schedule at all (FOREIGN KEY constraint failed).
--
-- SQLite can't drop a constraint in place, so rebuild the table without it and
-- carry over any existing rows. Columns are narrowed to the ones the scheduler
-- actually uses; next_run_at is derived from last_run_at + interval at query
-- time, and notify_on_critical lives in notification_config.
CREATE TABLE scan_schedules_new (
    cluster_id       TEXT PRIMARY KEY,
    enabled          BIGINT NOT NULL DEFAULT 0,
    interval_minutes BIGINT NOT NULL DEFAULT 60,
    last_run_at      BIGINT NOT NULL DEFAULT 0
);

INSERT INTO scan_schedules_new (cluster_id, enabled, interval_minutes, last_run_at)
SELECT cluster_id,
       COALESCE(enabled, 0),
       COALESCE(interval_minutes, 60),
       COALESCE(last_run_at, 0)
FROM scan_schedules;

DROP TABLE scan_schedules;

ALTER TABLE scan_schedules_new RENAME TO scan_schedules;
