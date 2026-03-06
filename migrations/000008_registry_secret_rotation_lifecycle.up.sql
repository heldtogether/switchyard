-- Allow historical (inactive) secret versions while preserving one active host/username per workspace.
ALTER TABLE registry_secrets
    DROP CONSTRAINT IF EXISTS registry_secrets_workspace_host_username_key;

DROP INDEX IF EXISTS idx_registry_secrets_workspace_host_active_unique;
CREATE UNIQUE INDEX idx_registry_secrets_workspace_host_active_unique
    ON registry_secrets(workspace_id, host, username)
    WHERE active = true;

ALTER TABLE registry_secrets
    ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deactivated_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS rotated_from_secret_id UUID REFERENCES registry_secrets(id) ON DELETE SET NULL;
