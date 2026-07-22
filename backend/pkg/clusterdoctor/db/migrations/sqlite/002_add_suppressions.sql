CREATE TABLE suppressions (
    cluster_id     TEXT NOT NULL,
    rule_id        TEXT NOT NULL,
    namespace      TEXT NOT NULL DEFAULT '',
    resource_kind  TEXT NOT NULL,
    resource_name  TEXT NOT NULL,
    reason         TEXT,
    suppressed_by  TEXT,
    suppressed_at  INTEGER NOT NULL,
    comment        TEXT,
    PRIMARY KEY (cluster_id, rule_id, namespace, resource_kind, resource_name)
);
