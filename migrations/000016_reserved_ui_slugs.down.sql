ALTER TABLE IF EXISTS projects
    DROP CONSTRAINT IF EXISTS projects_slug_not_reserved_ui_routes;

ALTER TABLE IF EXISTS workspaces
    DROP CONSTRAINT IF EXISTS workspaces_slug_not_reserved_ui_routes;
