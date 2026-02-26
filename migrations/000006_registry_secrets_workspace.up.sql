-- Migration: Add workspace scoping to registry secrets

-- Add workspace_id with default to the default workspace
ALTER TABLE registry_secrets
    ADD COLUMN workspace_id UUID;

UPDATE registry_secrets
SET workspace_id = '00000000-0000-0000-0000-000000000001'
WHERE workspace_id IS NULL;

ALTER TABLE registry_secrets
    ALTER COLUMN workspace_id SET NOT NULL,
    ADD CONSTRAINT fk_registry_secrets_workspace
        FOREIGN KEY (workspace_id)
        REFERENCES workspaces(id)
        ON DELETE CASCADE;

-- Replace unique constraint to include workspace scope
ALTER TABLE registry_secrets
    DROP CONSTRAINT IF EXISTS registry_secrets_host_username_key;

ALTER TABLE registry_secrets
    ADD CONSTRAINT registry_secrets_workspace_host_username_key
        UNIQUE (workspace_id, host, username);

-- Update index to include workspace scope
DROP INDEX IF EXISTS idx_registry_secrets_host;
CREATE INDEX idx_registry_secrets_workspace_host
    ON registry_secrets(workspace_id, host)
    WHERE active = true;
