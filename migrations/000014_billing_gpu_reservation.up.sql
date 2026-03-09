ALTER TABLE usage_events
    ADD COLUMN gpu_seconds NUMERIC(20,6) NOT NULL DEFAULT 0;

ALTER TABLE billing_ledger_entries
    ADD COLUMN gpu_seconds NUMERIC(20,6) NOT NULL DEFAULT 0,
    ADD COLUMN gpu_unit_price_minor BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN stripe_gpu_price_id TEXT,
    ADD COLUMN estimated_gpu_minor BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN estimated_gpu_minor_exact NUMERIC(20,9) NOT NULL DEFAULT 0;
