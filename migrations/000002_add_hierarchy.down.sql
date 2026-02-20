-- Rollback migration: Remove hierarchy and restore flat structure

-- Drop trigger and function for run status updates
DROP TRIGGER IF EXISTS trigger_update_run_status ON jobs;
DROP FUNCTION IF EXISTS update_run_status();

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS artefacts CASCADE;
DROP TABLE IF EXISTS jobs CASCADE;
DROP TABLE IF EXISTS runs CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
DROP TABLE IF EXISTS workspaces CASCADE;

-- Drop run status enum
DROP TYPE IF EXISTS run_status;

-- Restore original jobs table
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
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

-- Indexes for common queries
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_created_by ON jobs(created_by);
CREATE INDEX idx_jobs_created_at ON jobs(created_at DESC);
CREATE INDEX idx_jobs_executor_ref ON jobs(executor_ref) WHERE executor_ref IS NOT NULL;

-- Recreate updated timestamp trigger
CREATE TRIGGER jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Add foreign key for registry secrets
ALTER TABLE jobs ADD CONSTRAINT fk_jobs_registry_secret
    FOREIGN KEY (registry_secret_id)
    REFERENCES registry_secrets(id)
    ON DELETE SET NULL;

-- Restore artefacts table
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
