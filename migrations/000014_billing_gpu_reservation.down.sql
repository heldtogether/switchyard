ALTER TABLE billing_ledger_entries
    DROP COLUMN IF EXISTS estimated_gpu_minor_exact,
    DROP COLUMN IF EXISTS estimated_gpu_minor,
    DROP COLUMN IF EXISTS stripe_gpu_price_id,
    DROP COLUMN IF EXISTS gpu_unit_price_minor,
    DROP COLUMN IF EXISTS gpu_seconds;

ALTER TABLE usage_events
    DROP COLUMN IF EXISTS gpu_seconds;
