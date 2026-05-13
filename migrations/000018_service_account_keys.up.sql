CREATE TABLE service_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    principal_id UUID NOT NULL UNIQUE REFERENCES principals(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    disabled_at TIMESTAMPTZ,
    disabled_by VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL
);

CREATE INDEX idx_service_accounts_workspace ON service_accounts(workspace_id);

CREATE TRIGGER service_accounts_updated_at
    BEFORE UPDATE ON service_accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TABLE service_account_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_account_id UUID NOT NULL REFERENCES service_accounts(id) ON DELETE CASCADE,
    name VARCHAR(255),
    token_hash VARCHAR(128) UNIQUE NOT NULL,
    token_prefix VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoked_by VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL
);

CREATE INDEX idx_service_account_keys_account ON service_account_keys(service_account_id);
CREATE INDEX idx_service_account_keys_expires_at ON service_account_keys(expires_at);
