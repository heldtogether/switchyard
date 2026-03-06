CREATE TYPE executor_type_new AS ENUM ('docker', 'kube');

ALTER TABLE jobs
    ALTER COLUMN executor DROP DEFAULT,
    ALTER COLUMN executor TYPE executor_type_new
    USING (
        CASE
            WHEN executor::text = 'swarm' THEN 'docker'
            ELSE executor::text
        END
    )::executor_type_new,
    ALTER COLUMN executor SET DEFAULT 'docker';

ALTER TABLE nodes
    ALTER COLUMN executor DROP DEFAULT,
    ALTER COLUMN executor TYPE executor_type_new
    USING (
        CASE
            WHEN executor::text = 'swarm' THEN 'docker'
            ELSE executor::text
        END
    )::executor_type_new,
    ALTER COLUMN executor SET DEFAULT 'docker';

DROP TYPE executor_type;
ALTER TYPE executor_type_new RENAME TO executor_type;
