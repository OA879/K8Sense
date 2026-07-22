-- Per-cluster severity override for a rule (NULL = use the rule's default).
ALTER TABLE rule_overrides ADD COLUMN severity_override TEXT;

-- Per-cluster scheduled scan config + notification webhooks (Phase 3).
CREATE TABLE IF NOT EXISTS notification_config (
    cluster_id       TEXT PRIMARY KEY,
    slack_webhook    TEXT,
    teams_webhook    TEXT,
    notify_critical  INTEGER NOT NULL DEFAULT 1
);
