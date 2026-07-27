-- 006: un-fix / revert support. See the sqlite copy for details.
ALTER TABLE audit_log ADD COLUMN revert_action TEXT;
ALTER TABLE audit_log ADD COLUMN revert_payload TEXT;
ALTER TABLE audit_log ADD COLUMN reverted_at BIGINT;
ALTER TABLE audit_log ADD COLUMN revert_of TEXT;
