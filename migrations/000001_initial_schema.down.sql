-- Drop tables
DROP TABLE IF EXISTS artefacts CASCADE;
DROP TABLE IF EXISTS jobs CASCADE;
DROP TABLE IF EXISTS registry_secrets CASCADE;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at() CASCADE;

-- Drop types
DROP TYPE IF EXISTS executor_type;
DROP TYPE IF EXISTS job_status;

-- Drop extension (be careful in shared databases)
-- DROP EXTENSION IF EXISTS "uuid-ossp";
