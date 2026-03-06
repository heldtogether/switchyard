ALTER TABLE nodes
    ADD COLUMN gpu_device_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE gpu_allocations
    ADD COLUMN device_ids TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX idx_gpu_allocations_node_active
    ON gpu_allocations(node_id)
    WHERE released_at IS NULL;
