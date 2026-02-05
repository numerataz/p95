-- Drop run_continuations table and indexes
DROP INDEX IF EXISTS idx_continuations_timestamp;
DROP INDEX IF EXISTS idx_continuations_step;
DROP INDEX IF EXISTS idx_continuations_run;
DROP TABLE IF EXISTS run_continuations;
