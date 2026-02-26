ALTER TABLE nodes
    ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN stale_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_nodes_is_active ON nodes(is_active);
