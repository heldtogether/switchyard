ALTER TABLE jobs DROP CONSTRAINT IF EXISTS fk_jobs_assigned_node;

DROP TABLE IF EXISTS gpu_allocations;
DROP TABLE IF EXISTS nodes;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS assigned_node_id,
    DROP COLUMN IF EXISTS gpu_count;
