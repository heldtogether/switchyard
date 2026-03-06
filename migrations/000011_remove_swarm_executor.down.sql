CREATE TYPE executor_type_old AS ENUM ('swarm', 'kube', 'docker');

ALTER TABLE jobs
    ALTER COLUMN executor DROP DEFAULT,
    ALTER COLUMN executor TYPE executor_type_old
    USING executor::text::executor_type_old,
    ALTER COLUMN executor SET DEFAULT 'swarm';

ALTER TABLE nodes
    ALTER COLUMN executor DROP DEFAULT,
    ALTER COLUMN executor TYPE executor_type_old
    USING executor::text::executor_type_old,
    ALTER COLUMN executor SET DEFAULT 'swarm';

DROP TYPE executor_type;
ALTER TYPE executor_type_old RENAME TO executor_type;
