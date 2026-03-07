ALTER TABLE billing_ledger_entries
    ADD COLUMN estimated_cpu_minor_exact NUMERIC(20,9) NOT NULL DEFAULT 0,
    ADD COLUMN estimated_memory_minor_exact NUMERIC(20,9) NOT NULL DEFAULT 0,
    ADD COLUMN estimated_total_minor_exact NUMERIC(20,9) NOT NULL DEFAULT 0;

UPDATE billing_ledger_entries
SET
    estimated_cpu_minor_exact = estimated_cpu_minor,
    estimated_memory_minor_exact = estimated_memory_minor,
    estimated_total_minor_exact = estimated_total_minor;
