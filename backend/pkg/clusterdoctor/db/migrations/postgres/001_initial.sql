-- Postgres variant of 001_initial.sql. Generated from the SQLite schema;
-- INTEGER -> BIGINT so unix-second timestamps do not overflow int32 in 2038.
CREATE TABLE clusters (
    id              TEXT PRIMARY KEY,
    display_name    TEXT,
    kubeconfig_path TEXT NOT NULL,
    server_url      TEXT,
    archived        BIGINT DEFAULT 0,
    created_at      BIGINT NOT NULL
);

CREATE TABLE scans (
    id              TEXT PRIMARY KEY,
    cluster_id      TEXT NOT NULL,
    started_at      BIGINT NOT NULL,
    completed_at    BIGINT,
    status          TEXT NOT NULL,
    total_findings  BIGINT DEFAULT 0,
    critical_count  BIGINT DEFAULT 0,
    warning_count   BIGINT DEFAULT 0,
    info_count      BIGINT DEFAULT 0,
    skipped_checks  BIGINT DEFAULT 0,
    error_message   TEXT
);

CREATE INDEX idx_scans_cluster_id ON scans(cluster_id);

CREATE TABLE findings (
    id              TEXT PRIMARY KEY,
    scan_id         TEXT NOT NULL REFERENCES scans(id),
    rule_id         TEXT NOT NULL,
    rule_name       TEXT NOT NULL,
    severity        TEXT NOT NULL,
    category        TEXT NOT NULL,
    namespace       TEXT,
    resource_kind   TEXT NOT NULL,
    resource_name   TEXT NOT NULL,
    description     TEXT NOT NULL,
    remediation     TEXT NOT NULL,
    raw_object      TEXT,
    detected_at     BIGINT NOT NULL
);

CREATE INDEX idx_findings_scan_id ON findings(scan_id);

CREATE TABLE audit_log (
    id              TEXT PRIMARY KEY,
    actor           TEXT NOT NULL,
    action          TEXT NOT NULL,
    cluster_id      TEXT NOT NULL,
    namespace       TEXT,
    resource_kind   TEXT,
    resource_name   TEXT,
    payload         TEXT,
    result          TEXT NOT NULL,
    error           TEXT,
    performed_at    BIGINT NOT NULL
);

CREATE TABLE custom_rules (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    yaml_content    TEXT NOT NULL,
    enabled         BIGINT DEFAULT 1,
    imported_at     BIGINT NOT NULL
);

CREATE TABLE rule_overrides (
    cluster_id      TEXT NOT NULL,
    rule_id         TEXT NOT NULL,
    enabled         BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (cluster_id, rule_id)
);

CREATE TABLE scan_schedules (
    cluster_id          TEXT PRIMARY KEY REFERENCES clusters(id),
    enabled             BIGINT DEFAULT 0,
    interval_minutes    BIGINT DEFAULT 60,
    last_run_at         BIGINT,
    next_run_at         BIGINT,
    notify_on_critical  BIGINT DEFAULT 1
);

CREATE TABLE preferences (
    key             TEXT PRIMARY KEY,
    value           TEXT NOT NULL
);
