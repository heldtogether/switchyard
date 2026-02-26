-- Add GPU resource tracking and node allocations

ALTER TABLE jobs
    ADD COLUMN gpu_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN assigned_node_id TEXT;

-- Nodes table
CREATE TABLE nodes (
    node_id TEXT PRIMARY KEY,
    hostname TEXT,
    executor executor_type NOT NULL DEFAULT 'swarm',
    gpu_total INTEGER NOT NULL DEFAULT 0,
    last_heartbeat TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER nodes_updated_at
    BEFORE UPDATE ON nodes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- GPU allocations table
CREATE TABLE gpu_allocations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL REFERENCES nodes(node_id) ON DELETE CASCADE,
    gpu_count INTEGER NOT NULL,
    allocated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ,
    UNIQUE(job_id)
);

CREATE INDEX idx_gpu_allocations_node_id ON gpu_allocations(node_id);
CREATE INDEX idx_gpu_allocations_released_at ON gpu_allocations(released_at);

ALTER TABLE jobs
    ADD CONSTRAINT fk_jobs_assigned_node
    FOREIGN KEY (assigned_node_id)
    REFERENCES nodes(node_id)
    ON DELETE SET NULL;
