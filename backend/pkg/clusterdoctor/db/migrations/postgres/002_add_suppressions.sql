-- Postgres variant of 002_add_suppressions.sql. Generated from the SQLite schema;
-- INTEGER -> BIGINT so unix-second timestamps do not overflow int32 in 2038.
CREATE TABLE suppressions (
    cluster_id     TEXT NOT NULL,
    rule_id        TEXT NOT NULL,
    namespace      TEXT NOT NULL DEFAULT '',
    resource_kind  TEXT NOT NULL,
    resource_name  TEXT NOT NULL,
    reason         TEXT,
    suppressed_by  TEXT,
    suppressed_at  BIGINT NOT NULL,
    comment        TEXT,
    PRIMARY KEY (cluster_id, rule_id, namespace, resource_kind, resource_name)
);
