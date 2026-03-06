ALTER TABLE registry_secrets
    ADD COLUMN IF NOT EXISTS secret_encoding VARCHAR(32) NOT NULL DEFAULT 'plain',
    ADD COLUMN IF NOT EXISTS secret_key_id VARCHAR(128);

CREATE INDEX IF NOT EXISTS idx_registry_secrets_secret_encoding
    ON registry_secrets(secret_encoding);
