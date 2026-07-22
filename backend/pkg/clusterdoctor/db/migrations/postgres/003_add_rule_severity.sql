-- Postgres variant of 003_add_rule_severity.sql. Generated from the SQLite schema;
-- INTEGER -> BIGINT so unix-second timestamps do not overflow int32 in 2038.
-- Per-cluster severity override for a rule (NULL = use the rule's default).
ALTER TABLE rule_overrides ADD COLUMN severity_override TEXT;

-- Per-cluster scheduled scan config + notification webhooks (Phase 3).
CREATE TABLE IF NOT EXISTS notification_config (
    cluster_id       TEXT PRIMARY KEY,
    slack_webhook    TEXT,
    teams_webhook    TEXT,
    notify_critical  BIGINT NOT NULL DEFAULT 1
);
