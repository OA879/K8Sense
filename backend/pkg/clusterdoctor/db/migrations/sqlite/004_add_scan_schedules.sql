-- No-op. scan_schedules is already created in 001_initial.sql; the
-- CREATE TABLE IF NOT EXISTS that originally lived here silently did nothing
-- and masked the fact that the 001 definition carries an unusable foreign key.
-- Migration 005 rebuilds the table correctly. Kept as a numbered placeholder so
-- the applied-migration sequence stays contiguous.
SELECT 1;
