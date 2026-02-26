-- Migration rollback: Remove workspace scoping from registry secrets

DROP INDEX IF EXISTS idx_registry_secrets_workspace_host;

ALTER TABLE registry_secrets
    DROP CONSTRAINT IF EXISTS registry_secrets_workspace_host_username_key;

ALTER TABLE registry_secrets
    DROP CONSTRAINT IF EXISTS fk_registry_secrets_workspace;

ALTER TABLE registry_secrets
    ALTER COLUMN workspace_id DROP NOT NULL;

UPDATE registry_secrets
SET workspace_id = NULL;

ALTER TABLE registry_secrets
    DROP COLUMN workspace_id;

ALTER TABLE registry_secrets
    ADD CONSTRAINT registry_secrets_host_username_key
        UNIQUE (host, username);

CREATE INDEX idx_registry_secrets_host
    ON registry_secrets(host)
    WHERE active = true;
