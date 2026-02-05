-- Run Continuations
-- Tracks resume events for runs, including config changes and continuation points

CREATE TABLE run_continuations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    step BIGINT NOT NULL,                    -- Metric step at continuation
    timestamp TIMESTAMPTZ DEFAULT NOW(),     -- When the continuation occurred
    config_before JSONB,                     -- Config snapshot before continuation
    config_after JSONB,                      -- New config after continuation
    note TEXT,                               -- Optional user note
    git_info JSONB,                          -- Git info at continuation time
    system_info JSONB,                       -- System info at continuation time
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for efficient lookup of continuations by run
CREATE INDEX idx_continuations_run ON run_continuations(run_id);

-- Index for efficient ordering by step within a run
CREATE INDEX idx_continuations_step ON run_continuations(run_id, step);

-- Index for timestamp-based queries
CREATE INDEX idx_continuations_timestamp ON run_continuations(run_id, timestamp);
