-- Sixtyseven TimescaleDB Hypertables for Metrics
-- Requires TimescaleDB extension

-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- ============================================================================
-- Metrics Table (main time-series data)
-- ============================================================================
-- Stores numeric metrics like loss, accuracy, learning_rate, etc.
CREATE TABLE metrics (
    time TIMESTAMPTZ NOT NULL,
    run_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    step BIGINT NOT NULL,
    value DOUBLE PRECISION NOT NULL
);

-- Convert to hypertable with time-based partitioning
-- Chunks are created per day for efficient querying and compression
SELECT create_hypertable('metrics', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Create composite indexes for efficient queries
-- Primary access pattern: get metrics for a run by name, ordered by time/step
CREATE INDEX idx_metrics_run_name_time ON metrics(run_id, name, time DESC);
CREATE INDEX idx_metrics_run_name_step ON metrics(run_id, name, step);

-- Enable compression for older data (90%+ storage savings)
ALTER TABLE metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'run_id, name',
    timescaledb.compress_orderby = 'time DESC'
);

-- Add compression policy (compress chunks older than 7 days)
SELECT add_compression_policy('metrics', INTERVAL '7 days');

-- ============================================================================
-- System Metrics Table (GPU usage, memory, etc.)
-- ============================================================================
CREATE TABLE system_metrics (
    time TIMESTAMPTZ NOT NULL,
    run_id UUID NOT NULL,
    metric_type VARCHAR(50) NOT NULL,  -- gpu_memory, gpu_utilization, cpu, memory
    device_id INTEGER DEFAULT 0,  -- For multi-GPU setups
    value DOUBLE PRECISION NOT NULL
);

SELECT create_hypertable('system_metrics', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

CREATE INDEX idx_system_metrics_run ON system_metrics(run_id, metric_type, time DESC);

-- Enable compression for system metrics
ALTER TABLE system_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'run_id, metric_type',
    timescaledb.compress_orderby = 'time DESC'
);

SELECT add_compression_policy('system_metrics', INTERVAL '7 days');

-- ============================================================================
-- Continuous Aggregates for Dashboard Summaries
-- ============================================================================
-- Pre-computed hourly aggregates for faster dashboard queries
CREATE MATERIALIZED VIEW metrics_hourly
WITH (timescaledb.continuous) AS
SELECT
    run_id,
    name,
    time_bucket('1 hour', time) AS bucket,
    AVG(value) AS avg_value,
    MIN(value) AS min_value,
    MAX(value) AS max_value,
    FIRST(value, time) AS first_value,
    LAST(value, time) AS last_value,
    COUNT(*) AS sample_count
FROM metrics
GROUP BY run_id, name, bucket
WITH NO DATA;

-- Refresh policy for continuous aggregate
-- Refreshes data from 3 hours ago to 1 hour ago, every hour
SELECT add_continuous_aggregate_policy('metrics_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour'
);

-- ============================================================================
-- Helper Views
-- ============================================================================

-- Latest metric values for a run (useful for dashboards)
CREATE VIEW run_latest_metrics AS
SELECT DISTINCT ON (run_id, name)
    run_id,
    name,
    value,
    step,
    time
FROM metrics
ORDER BY run_id, name, time DESC;

-- Run statistics summary
CREATE VIEW run_stats AS
SELECT
    run_id,
    COUNT(DISTINCT name) AS metric_count,
    COUNT(*) AS total_points,
    MIN(time) AS first_metric_at,
    MAX(time) AS last_metric_at,
    MAX(step) AS max_step
FROM metrics
GROUP BY run_id;

-- ============================================================================
-- Retention Policy (optional, uncomment for cloud tier management)
-- ============================================================================
-- Automatically delete data older than 90 days (for free tier)
-- SELECT add_retention_policy('metrics', INTERVAL '90 days');
-- SELECT add_retention_policy('system_metrics', INTERVAL '90 days');
