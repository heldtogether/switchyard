DROP INDEX IF EXISTS idx_stripe_meter_events_retry;
DROP TRIGGER IF EXISTS stripe_meter_events_updated_at ON stripe_meter_events;
DROP TABLE IF EXISTS stripe_meter_events;
DROP TYPE IF EXISTS stripe_meter_event_status;

DROP INDEX IF EXISTS idx_billing_ledger_run_id;
DROP INDEX IF EXISTS idx_billing_ledger_workspace_month;
DROP TABLE IF EXISTS billing_ledger_entries;

DROP INDEX IF EXISTS idx_usage_events_run_id;
DROP INDEX IF EXISTS idx_usage_events_workspace_month;
DROP TABLE IF EXISTS usage_events;

DROP TRIGGER IF EXISTS workspace_billing_accounts_updated_at ON workspace_billing_accounts;
DROP TABLE IF EXISTS workspace_billing_accounts;
