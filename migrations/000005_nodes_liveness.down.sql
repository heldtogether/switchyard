DROP INDEX IF EXISTS idx_nodes_is_active;
ALTER TABLE nodes
    DROP COLUMN IF EXISTS stale_at,
    DROP COLUMN IF EXISTS is_active;
