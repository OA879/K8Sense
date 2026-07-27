-- 006: un-fix / revert support.
-- A guided-fix audit entry can now carry the information needed to reverse it
-- (revert_action + the prior state in revert_payload). reverted_at is stamped
-- when the entry has been undone; a revert entry points at the original via
-- revert_of. Non-reversible actions (delete_pod/job, restart) leave
-- revert_action NULL.
ALTER TABLE audit_log ADD COLUMN revert_action TEXT;
ALTER TABLE audit_log ADD COLUMN revert_payload TEXT;
ALTER TABLE audit_log ADD COLUMN reverted_at INTEGER;
ALTER TABLE audit_log ADD COLUMN revert_of TEXT;
