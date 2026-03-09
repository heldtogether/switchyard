DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_enum e
        JOIN pg_type t ON t.oid = e.enumtypid
        WHERE t.typname = 'job_status' AND e.enumlabel = 'CANCELLING'
    ) THEN
        ALTER TYPE job_status ADD VALUE 'CANCELLING';
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_enum e
        JOIN pg_type t ON t.oid = e.enumtypid
        WHERE t.typname = 'run_status' AND e.enumlabel = 'CANCELLING'
    ) THEN
        ALTER TYPE run_status ADD VALUE 'CANCELLING';
    END IF;
END $$;

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS cancel_requested_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancel_requested_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS cancel_reason TEXT;

CREATE TABLE IF NOT EXISTS job_cancellation_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    event_type VARCHAR(32) NOT NULL,
    requested_by VARCHAR(255) NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    node_id VARCHAR(255),
    executor_ref VARCHAR(255),
    message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (event_type IN ('requested', 'dispatched', 'acknowledged', 'completed', 'failed', 'skipped_terminal'))
);

CREATE INDEX IF NOT EXISTS idx_job_cancellation_events_job_id_created_at
    ON job_cancellation_events(job_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_cancellation_events_run_id_created_at
    ON job_cancellation_events(run_id, created_at DESC);

CREATE OR REPLACE FUNCTION update_run_status()
RETURNS TRIGGER AS $$
DECLARE
    v_run_id UUID;
    v_pending_count INTEGER;
    v_cancelling_count INTEGER;
    v_running_count INTEGER;
    v_succeeded_count INTEGER;
    v_failed_count INTEGER;
    v_cancelled_count INTEGER;
    v_timeout_count INTEGER;
    v_total_count INTEGER;
    v_new_status run_status;
    v_min_started_at TIMESTAMPTZ;
    v_max_finished_at TIMESTAMPTZ;
BEGIN
    IF TG_OP = 'DELETE' THEN
        v_run_id := OLD.run_id;
    ELSE
        v_run_id := NEW.run_id;
    END IF;

    SELECT
        COUNT(*) FILTER (WHERE status = 'PENDING'),
        COUNT(*) FILTER (WHERE status = 'CANCELLING'),
        COUNT(*) FILTER (WHERE status = 'RUNNING'),
        COUNT(*) FILTER (WHERE status = 'SUCCEEDED'),
        COUNT(*) FILTER (WHERE status = 'FAILED'),
        COUNT(*) FILTER (WHERE status = 'CANCELLED'),
        COUNT(*) FILTER (WHERE status = 'TIMEOUT'),
        COUNT(*),
        MIN(started_at),
        MAX(finished_at)
    INTO
        v_pending_count,
        v_cancelling_count,
        v_running_count,
        v_succeeded_count,
        v_failed_count,
        v_cancelled_count,
        v_timeout_count,
        v_total_count,
        v_min_started_at,
        v_max_finished_at
    FROM jobs
    WHERE run_id = v_run_id;

    IF v_total_count = 0 THEN
        v_new_status := 'PENDING';
    ELSIF v_cancelling_count > 0 THEN
        v_new_status := 'CANCELLING';
    ELSIF v_running_count > 0 THEN
        v_new_status := 'RUNNING';
    ELSIF v_cancelled_count = v_total_count THEN
        v_new_status := 'CANCELLED';
    ELSIF v_succeeded_count = v_total_count THEN
        v_new_status := 'SUCCEEDED';
    ELSIF (v_failed_count + v_timeout_count) = v_total_count THEN
        v_new_status := 'FAILED';
    ELSIF v_pending_count = 0 THEN
        v_new_status := 'PARTIAL';
    ELSE
        v_new_status := 'RUNNING';
    END IF;

    UPDATE runs
    SET
        status = v_new_status,
        started_at = COALESCE(started_at, v_min_started_at),
        finished_at = CASE
            WHEN v_new_status IN ('SUCCEEDED', 'FAILED', 'CANCELLED', 'PARTIAL')
            THEN v_max_finished_at
            ELSE NULL
        END
    WHERE id = v_run_id;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
