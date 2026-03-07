-- Billing-grade usage metering and Stripe meter event outbox

CREATE TABLE workspace_billing_accounts (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    stripe_customer_id TEXT NOT NULL UNIQUE,
    stripe_subscription_id TEXT,
    invoices_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER workspace_billing_accounts_updated_at
    BEFORE UPDATE ON workspace_billing_accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TABLE usage_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    container_id TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NOT NULL,
    duration_seconds NUMERIC(20,6) NOT NULL,
    cpu_seconds NUMERIC(20,6) NOT NULL,
    memory_gb_seconds NUMERIC(20,9) NOT NULL,
    max_memory_bytes BIGINT NOT NULL,
    sample_interval_seconds INTEGER NOT NULL,
    source TEXT NOT NULL DEFAULT 'docker_stats',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(job_id)
);

CREATE INDEX idx_usage_events_workspace_month
    ON usage_events(workspace_id, finished_at DESC);

CREATE INDEX idx_usage_events_run_id
    ON usage_events(run_id, finished_at DESC);

CREATE TABLE billing_ledger_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    usage_event_id UUID NOT NULL UNIQUE REFERENCES usage_events(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    month_key CHAR(7) NOT NULL,
    cpu_seconds NUMERIC(20,6) NOT NULL,
    memory_gb_seconds NUMERIC(20,9) NOT NULL,
    pricing_version TEXT NOT NULL,
    currency CHAR(3) NOT NULL,
    cpu_unit_price_minor BIGINT NOT NULL,
    memory_unit_price_minor BIGINT NOT NULL,
    stripe_cpu_price_id TEXT,
    stripe_memory_price_id TEXT,
    estimated_cpu_minor BIGINT NOT NULL,
    estimated_memory_minor BIGINT NOT NULL,
    estimated_total_minor BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_billing_ledger_workspace_month
    ON billing_ledger_entries(workspace_id, month_key, created_at DESC);

CREATE INDEX idx_billing_ledger_run_id
    ON billing_ledger_entries(workspace_id, run_id, created_at DESC);

CREATE TYPE stripe_meter_event_status AS ENUM ('pending', 'sent', 'failed', 'blocked');

CREATE TABLE stripe_meter_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    usage_event_id UUID NOT NULL REFERENCES usage_events(id) ON DELETE CASCADE,
    meter_name TEXT NOT NULL,
    meter_value NUMERIC(20,9) NOT NULL,
    event_timestamp TIMESTAMPTZ NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    status stripe_meter_event_status NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_attempt_at TIMESTAMPTZ,
    last_error TEXT,
    stripe_event_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER stripe_meter_events_updated_at
    BEFORE UPDATE ON stripe_meter_events
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE INDEX idx_stripe_meter_events_retry
    ON stripe_meter_events(status, next_retry_at ASC);

