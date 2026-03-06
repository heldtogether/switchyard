DROP INDEX IF EXISTS idx_registry_secrets_secret_encoding;

ALTER TABLE registry_secrets
    DROP COLUMN IF EXISTS secret_key_id,
    DROP COLUMN IF EXISTS secret_encoding;
