DROP INDEX IF EXISTS idx_gpu_allocations_node_active;

ALTER TABLE gpu_allocations
    DROP COLUMN IF EXISTS device_ids;

ALTER TABLE nodes
    DROP COLUMN IF EXISTS gpu_device_ids;
