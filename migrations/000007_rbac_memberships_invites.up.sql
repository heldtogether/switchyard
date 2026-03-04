-- RBAC principals, memberships, and tokenized invites.

CREATE TYPE member_role AS ENUM ('owner', 'member');

CREATE TABLE principals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subject VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(320),
    display_name VARCHAR(255),
    provider VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_principals_email ON principals(email);

CREATE TRIGGER principals_updated_at
    BEFORE UPDATE ON principals
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TABLE workspace_memberships (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    principal_id UUID NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    role member_role NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    PRIMARY KEY (workspace_id, principal_id)
);

CREATE INDEX idx_workspace_memberships_principal ON workspace_memberships(principal_id);

CREATE TABLE project_memberships (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    principal_id UUID NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    role member_role NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    PRIMARY KEY (project_id, principal_id)
);

CREATE INDEX idx_project_memberships_principal ON project_memberships(principal_id);

CREATE TABLE workspace_invites (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email VARCHAR(320) NOT NULL,
    role member_role NOT NULL DEFAULT 'member',
    token_hash VARCHAR(128) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL
);

CREATE INDEX idx_workspace_invites_workspace ON workspace_invites(workspace_id);
CREATE INDEX idx_workspace_invites_email ON workspace_invites(email);

CREATE TABLE project_invites (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    email VARCHAR(320) NOT NULL,
    role member_role NOT NULL DEFAULT 'member',
    token_hash VARCHAR(128) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL
);

CREATE INDEX idx_project_invites_project ON project_invites(project_id);
CREATE INDEX idx_project_invites_email ON project_invites(email);
