DROP INDEX IF EXISTS idx_project_channel_promotions_project;
DROP TRIGGER IF EXISTS project_channel_promotions_updated_at ON project_channel_promotions;
DROP TABLE IF EXISTS project_channel_promotions;

DROP INDEX IF EXISTS idx_promotion_event_artefacts_logical;
DROP INDEX IF EXISTS idx_promotion_event_artefacts_event;
DROP TABLE IF EXISTS promotion_event_artefacts;

DROP INDEX IF EXISTS idx_promotion_events_project_channel_created;
DROP TABLE IF EXISTS promotion_events;

DROP TYPE IF EXISTS promotion_channel;
