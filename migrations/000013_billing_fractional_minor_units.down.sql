ALTER TABLE billing_ledger_entries
    DROP COLUMN IF EXISTS estimated_total_minor_exact,
    DROP COLUMN IF EXISTS estimated_memory_minor_exact,
    DROP COLUMN IF EXISTS estimated_cpu_minor_exact;
