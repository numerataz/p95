-- Rollback TimescaleDB hypertables

-- Remove policies first
SELECT remove_continuous_aggregate_policy('metrics_hourly', if_exists => true);
SELECT remove_compression_policy('system_metrics', if_exists => true);
SELECT remove_compression_policy('metrics', if_exists => true);

-- Drop views
DROP VIEW IF EXISTS run_stats;
DROP VIEW IF EXISTS run_latest_metrics;

-- Drop continuous aggregate
DROP MATERIALIZED VIEW IF EXISTS metrics_hourly;

-- Drop hypertables
DROP TABLE IF EXISTS system_metrics;
DROP TABLE IF EXISTS metrics;
