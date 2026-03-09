DROP TABLE IF EXISTS job_cancellation_events;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS cancel_reason,
    DROP COLUMN IF EXISTS cancel_requested_by,
    DROP COLUMN IF EXISTS cancel_requested_at;

CREATE OR REPLACE FUNCTION update_run_status()
RETURNS TRIGGER AS $$
DECLARE
    v_run_id UUID;
    v_pending_count INTEGER;
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

-- enum values are intentionally not removed in down migration
