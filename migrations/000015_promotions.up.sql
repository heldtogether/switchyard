CREATE TYPE promotion_channel AS ENUM ('dev', 'staging', 'prod', 'validated');

CREATE TABLE promotion_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    channel promotion_channel NOT NULL,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    note TEXT,
    promoted_by VARCHAR(255) NOT NULL,
    promoted_by_principal_id UUID REFERENCES principals(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_promotion_events_project_channel_created
    ON promotion_events(project_id, channel, created_at DESC);

CREATE TABLE promotion_event_artefacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    promotion_event_id UUID NOT NULL REFERENCES promotion_events(id) ON DELETE CASCADE,
    logical_key VARCHAR(128) NOT NULL,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    path VARCHAR(512) NOT NULL,
    object_key VARCHAR(512) NOT NULL,
    size_bytes BIGINT NOT NULL,
    content_type VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (promotion_event_id, logical_key)
);

CREATE INDEX idx_promotion_event_artefacts_event
    ON promotion_event_artefacts(promotion_event_id);

CREATE INDEX idx_promotion_event_artefacts_logical
    ON promotion_event_artefacts(logical_key);

CREATE TABLE project_channel_promotions (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    channel promotion_channel NOT NULL,
    promotion_event_id UUID NOT NULL REFERENCES promotion_events(id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, channel)
);

CREATE TRIGGER project_channel_promotions_updated_at
    BEFORE UPDATE ON project_channel_promotions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE INDEX idx_project_channel_promotions_project
    ON project_channel_promotions(project_id);
