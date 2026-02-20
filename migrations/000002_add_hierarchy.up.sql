-- Migration: Add Workspace → Project → Run → Job hierarchy
-- This migration introduces organizational hierarchy for multi-tenancy and better job organization

-- ============================================================================
-- Step 1: Create Workspaces table
-- ============================================================================
CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_workspaces_slug ON workspaces(slug);

CREATE TRIGGER workspaces_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Insert default workspace
INSERT INTO workspaces (id, slug, name, description) VALUES
    ('00000000-0000-0000-0000-000000000001', 'default', 'Default Workspace', 'Default workspace for single-tenant usage');

-- ============================================================================
-- Step 2: Create Projects table
-- ============================================================================
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slug VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    archived BOOLEAN DEFAULT false,
    metadata JSONB DEFAULT '{}',
    
    UNIQUE(workspace_id, slug)
);

CREATE INDEX idx_projects_workspace_id ON projects(workspace_id);
CREATE INDEX idx_projects_slug ON projects(workspace_id, slug);
CREATE INDEX idx_projects_archived ON projects(archived) WHERE archived = false;

CREATE TRIGGER projects_updated_at
    BEFORE UPDATE ON projects
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- ============================================================================
-- Step 3: Create Run status enum and Runs table
-- ============================================================================
CREATE TYPE run_status AS ENUM (
    'PENDING',      -- Run created, jobs not yet started
    'RUNNING',      -- At least one job running
    'SUCCEEDED',    -- All jobs succeeded
    'FAILED',       -- At least one job failed
    'CANCELLED',    -- Run was cancelled
    'PARTIAL'       -- Some jobs succeeded, some failed
);

CREATE TABLE runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    slug VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status run_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_by VARCHAR(255) NOT NULL,
    metadata JSONB DEFAULT '{}',
    
    UNIQUE(project_id, slug)
);

CREATE INDEX idx_runs_project_id ON runs(project_id);
CREATE INDEX idx_runs_slug ON runs(project_id, slug);
CREATE INDEX idx_runs_status ON runs(status);
CREATE INDEX idx_runs_created_at ON runs(created_at DESC);

CREATE TRIGGER runs_updated_at
    BEFORE UPDATE ON runs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- ============================================================================
-- Step 4: Backup and drop old jobs table
-- ============================================================================
-- Drop old jobs table (fresh start as per requirements)
DROP TABLE IF EXISTS artefacts CASCADE;
DROP TABLE IF EXISTS jobs CASCADE;

-- ============================================================================
-- Step 5: Create new Jobs table with run_id
-- ============================================================================
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    
    -- Status tracking
    status job_status NOT NULL DEFAULT 'PENDING',
    status_message TEXT,
    
    -- Container specification
    image VARCHAR(512) NOT NULL,
    image_digest VARCHAR(128),
    command JSONB,
    env JSONB DEFAULT '{}',
    
    -- Resources and limits
    cpu_limit VARCHAR(10),
    memory_limit VARCHAR(20),
    timeout_seconds INTEGER DEFAULT 3600,
    
    -- Outputs configuration
    outputs JSONB NOT NULL DEFAULT '[]',
    
    -- Execution tracking
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    exit_code INTEGER,
    
    -- Storage references
    artefact_prefix VARCHAR(512),
    log_object_key VARCHAR(512),
    
    -- Executor details
    executor executor_type NOT NULL DEFAULT 'swarm',
    executor_ref VARCHAR(255),
    executor_metadata JSONB DEFAULT '{}',
    
    -- Registry auth reference
    registry_secret_id UUID,
    
    -- Extensibility
    metadata JSONB DEFAULT '{}'
);

-- Indexes for jobs
CREATE INDEX idx_jobs_run_id ON jobs(run_id);
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_created_by ON jobs(created_by);
CREATE INDEX idx_jobs_created_at ON jobs(created_at DESC);
CREATE INDEX idx_jobs_executor_ref ON jobs(executor_ref) WHERE executor_ref IS NOT NULL;

CREATE TRIGGER jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Add foreign key for registry secrets
ALTER TABLE jobs ADD CONSTRAINT fk_jobs_registry_secret
    FOREIGN KEY (registry_secret_id)
    REFERENCES registry_secrets(id)
    ON DELETE SET NULL;

-- ============================================================================
-- Step 6: Recreate Artefacts table
-- ============================================================================
CREATE TABLE artefacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    path VARCHAR(512) NOT NULL,
    object_key VARCHAR(512) NOT NULL,
    size_bytes BIGINT NOT NULL,
    content_type VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(job_id, path)
);

CREATE INDEX idx_artefacts_job_id ON artefacts(job_id);

-- ============================================================================
-- Step 7: Create helper function to update run status based on jobs
-- ============================================================================
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
    -- Get run_id from NEW or OLD record
    IF TG_OP = 'DELETE' THEN
        v_run_id := OLD.run_id;
    ELSE
        v_run_id := NEW.run_id;
    END IF;

    -- Count job statuses for this run
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

    -- Determine new run status
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

    -- Update run status and timestamps
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

-- Trigger to update run status when job status changes
CREATE TRIGGER trigger_update_run_status
    AFTER INSERT OR UPDATE OF status OR DELETE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_run_status();
