CREATE TABLE clusters (
    id              TEXT PRIMARY KEY,
    display_name    TEXT,
    kubeconfig_path TEXT NOT NULL,
    server_url      TEXT,
    archived        INTEGER DEFAULT 0,
    created_at      INTEGER NOT NULL
);

CREATE TABLE scans (
    id              TEXT PRIMARY KEY,
    cluster_id      TEXT NOT NULL,
    started_at      INTEGER NOT NULL,
    completed_at    INTEGER,
    status          TEXT NOT NULL,
    total_findings  INTEGER DEFAULT 0,
    critical_count  INTEGER DEFAULT 0,
    warning_count   INTEGER DEFAULT 0,
    info_count      INTEGER DEFAULT 0,
    skipped_checks  INTEGER DEFAULT 0,
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
    detected_at     INTEGER NOT NULL
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
    performed_at    INTEGER NOT NULL
);

CREATE TABLE custom_rules (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    yaml_content    TEXT NOT NULL,
    enabled         INTEGER DEFAULT 1,
    imported_at     INTEGER NOT NULL
);

CREATE TABLE rule_overrides (
    cluster_id      TEXT NOT NULL,
    rule_id         TEXT NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (cluster_id, rule_id)
);

CREATE TABLE scan_schedules (
    cluster_id          TEXT PRIMARY KEY REFERENCES clusters(id),
    enabled             INTEGER DEFAULT 0,
    interval_minutes    INTEGER DEFAULT 60,
    last_run_at         INTEGER,
    next_run_at         INTEGER,
    notify_on_critical  INTEGER DEFAULT 1
);

CREATE TABLE preferences (
    key             TEXT PRIMARY KEY,
    value           TEXT NOT NULL
);
