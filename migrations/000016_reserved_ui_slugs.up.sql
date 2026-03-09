DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM workspaces
        WHERE lower(slug) = ANY (ARRAY[
            'runs',
            'jobs',
            'artefacts',
            'billing',
            'executors',
            'settings',
            'login',
            'accept-invite',
            'api'
        ])
    ) THEN
        RAISE EXCEPTION 'Migration 000016 blocked: existing workspace slug conflicts with reserved UI route. Rename conflicting workspace slug(s) before retrying.';
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM projects
        WHERE lower(slug) = ANY (ARRAY[
            'runs',
            'jobs',
            'artefacts',
            'billing',
            'executors',
            'settings',
            'login',
            'accept-invite',
            'api'
        ])
    ) THEN
        RAISE EXCEPTION 'Migration 000016 blocked: existing project slug conflicts with reserved UI route. Rename conflicting project slug(s) before retrying.';
    END IF;
END $$;

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_slug_not_reserved_ui_routes
    CHECK (
        NOT (
            lower(slug) = ANY (ARRAY[
                'runs',
                'jobs',
                'artefacts',
                'billing',
                'executors',
                'settings',
                'login',
                'accept-invite',
                'api'
            ])
        )
    );

ALTER TABLE projects
    ADD CONSTRAINT projects_slug_not_reserved_ui_routes
    CHECK (
        NOT (
            lower(slug) = ANY (ARRAY[
                'runs',
                'jobs',
                'artefacts',
                'billing',
                'executors',
                'settings',
                'login',
                'accept-invite',
                'api'
            ])
        )
    );
