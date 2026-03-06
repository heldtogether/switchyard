-- Remove lifecycle columns and restore strict uniqueness.
ALTER TABLE registry_secrets
    DROP COLUMN IF EXISTS rotated_from_secret_id,
    DROP COLUMN IF EXISTS deactivated_by,
    DROP COLUMN IF EXISTS deactivated_at;

DROP INDEX IF EXISTS idx_registry_secrets_workspace_host_active_unique;

ALTER TABLE registry_secrets
    ADD CONSTRAINT registry_secrets_workspace_host_username_key
        UNIQUE (workspace_id, host, username);
