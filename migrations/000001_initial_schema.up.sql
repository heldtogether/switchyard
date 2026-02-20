-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Job statuses enum
CREATE TYPE job_status AS ENUM (
    'PENDING',
    'RUNNING',
    'SUCCEEDED',
    'FAILED',
    'CANCELLED',
    'TIMEOUT'
);

-- Executor types enum
CREATE TYPE executor_type AS ENUM ('swarm', 'kube');

-- Jobs table
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

-- Updated timestamp trigger
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER jobs_updated_at
    BEFORE UPDATE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Artefacts table
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

-- Registry secrets table
CREATE TABLE registry_secrets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    
    host VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL,
    password_encrypted TEXT NOT NULL,
    
    active BOOLEAN DEFAULT true,
    
    UNIQUE(host, username)
);

CREATE INDEX idx_registry_secrets_host ON registry_secrets(host) WHERE active = true;

-- Add foreign key for registry secrets
ALTER TABLE jobs ADD CONSTRAINT fk_jobs_registry_secret
    FOREIGN KEY (registry_secret_id)
    REFERENCES registry_secrets(id)
    ON DELETE SET NULL;
