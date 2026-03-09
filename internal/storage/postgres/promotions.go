package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/lib/pq"
)

var ErrPromotionNotFound = fmt.Errorf("promotion not found")

type PromotionEventCreateArtefactInput struct {
	LogicalKey string
	JobID      uuid.UUID
	Path       string
}

type PromotionEventCreateInput struct {
	WorkspaceID           uuid.UUID
	ProjectID             uuid.UUID
	Channel               domain.PromotionChannel
	RunID                 uuid.UUID
	Note                  *string
	PromotedBy            string
	PromotedByPrincipalID *uuid.UUID
	Artefacts             []PromotionEventCreateArtefactInput
}

func (s *Store) CreatePromotionEventAndSetCurrent(ctx context.Context, input PromotionEventCreateInput) (*domain.PromotionEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	event := &domain.PromotionEvent{
		ID:                    uuid.New(),
		WorkspaceID:           input.WorkspaceID,
		ProjectID:             input.ProjectID,
		Channel:               input.Channel,
		RunID:                 input.RunID,
		Note:                  input.Note,
		PromotedBy:            input.PromotedBy,
		PromotedByPrincipalID: input.PromotedByPrincipalID,
		Artefacts:             make([]domain.PromotionArtefactRef, 0, len(input.Artefacts)),
	}

	if err := tx.QueryRowContext(ctx, `
		INSERT INTO promotion_events (
			id, workspace_id, project_id, channel, run_id, note, promoted_by, promoted_by_principal_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at
	`, event.ID, event.WorkspaceID, event.ProjectID, event.Channel, event.RunID, event.Note, event.PromotedBy, event.PromotedByPrincipalID).Scan(&event.CreatedAt); err != nil {
		return nil, err
	}

	for _, art := range input.Artefacts {
		resolved := domain.PromotionArtefactRef{
			LogicalKey: strings.ToLower(strings.TrimSpace(art.LogicalKey)),
			JobID:      art.JobID,
			Path:       art.Path,
		}

		if err := tx.QueryRowContext(ctx, `
			SELECT a.object_key, a.size_bytes, COALESCE(a.content_type, '')
			FROM artefacts a
			JOIN jobs j ON j.id = a.job_id
			JOIN runs r ON r.id = j.run_id
			WHERE a.job_id = $1 AND a.path = $2 AND r.id = $3 AND r.project_id = $4
		`, art.JobID, art.Path, input.RunID, input.ProjectID).Scan(&resolved.ObjectKey, &resolved.SizeBytes, &resolved.ContentType); err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("artefact not found in run: job=%s path=%s", art.JobID, art.Path)
			}
			return nil, err
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO promotion_event_artefacts (
				promotion_event_id, logical_key, job_id, path, object_key, size_bytes, content_type
			) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
		`, event.ID, resolved.LogicalKey, resolved.JobID, resolved.Path, resolved.ObjectKey, resolved.SizeBytes, resolved.ContentType); err != nil {
			return nil, err
		}

		event.Artefacts = append(event.Artefacts, resolved)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_channel_promotions (project_id, channel, promotion_event_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, channel) DO UPDATE
		SET promotion_event_id = EXCLUDED.promotion_event_id,
		    updated_at = NOW()
	`, input.ProjectID, input.Channel, event.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *Store) ListCurrentPromotions(ctx context.Context, workspaceID, projectID uuid.UUID) ([]domain.CurrentChannelPromotion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pcp.project_id,
		       pcp.channel,
		       pe.id,
		       pe.workspace_id,
		       pe.project_id,
		       pe.channel,
		       pe.run_id,
		       pe.note,
		       pe.promoted_by,
		       pe.promoted_by_principal_id,
		       pe.created_at
		FROM project_channel_promotions pcp
		JOIN promotion_events pe ON pe.id = pcp.promotion_event_id
		WHERE pe.workspace_id = $1
		  AND pcp.project_id = $2
		ORDER BY pcp.channel ASC
	`, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.CurrentChannelPromotion, 0)
	eventIDs := make([]uuid.UUID, 0)

	for rows.Next() {
		var item domain.CurrentChannelPromotion
		if err := rows.Scan(
			&item.ProjectID,
			&item.Channel,
			&item.Event.ID,
			&item.Event.WorkspaceID,
			&item.Event.ProjectID,
			&item.Event.Channel,
			&item.Event.RunID,
			&item.Event.Note,
			&item.Event.PromotedBy,
			&item.Event.PromotedByPrincipalID,
			&item.Event.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
		eventIDs = append(eventIDs, item.Event.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	artefactsByEvent, err := s.loadPromotionArtefactsByEventID(ctx, eventIDs)
	if err != nil {
		return nil, err
	}
	for i := range result {
		result[i].Event.Artefacts = artefactsByEvent[result[i].Event.ID]
	}

	return result, nil
}

func (s *Store) GetCurrentPromotionByChannel(ctx context.Context, workspaceID, projectID uuid.UUID, channel domain.PromotionChannel) (*domain.CurrentChannelPromotion, error) {
	item := &domain.CurrentChannelPromotion{}
	err := s.db.QueryRowContext(ctx, `
		SELECT pcp.project_id,
		       pcp.channel,
		       pe.id,
		       pe.workspace_id,
		       pe.project_id,
		       pe.channel,
		       pe.run_id,
		       pe.note,
		       pe.promoted_by,
		       pe.promoted_by_principal_id,
		       pe.created_at
		FROM project_channel_promotions pcp
		JOIN promotion_events pe ON pe.id = pcp.promotion_event_id
		WHERE pe.workspace_id = $1
		  AND pcp.project_id = $2
		  AND pcp.channel = $3
	`, workspaceID, projectID, channel).Scan(
		&item.ProjectID,
		&item.Channel,
		&item.Event.ID,
		&item.Event.WorkspaceID,
		&item.Event.ProjectID,
		&item.Event.Channel,
		&item.Event.RunID,
		&item.Event.Note,
		&item.Event.PromotedBy,
		&item.Event.PromotedByPrincipalID,
		&item.Event.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrPromotionNotFound
	}
	if err != nil {
		return nil, err
	}

	artefactsByEvent, err := s.loadPromotionArtefactsByEventID(ctx, []uuid.UUID{item.Event.ID})
	if err != nil {
		return nil, err
	}
	item.Event.Artefacts = artefactsByEvent[item.Event.ID]

	return item, nil
}

func (s *Store) ListPromotionHistory(ctx context.Context, workspaceID, projectID uuid.UUID, channel *domain.PromotionChannel, limit, offset int) ([]domain.PromotionEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, project_id, channel, run_id, note, promoted_by, promoted_by_principal_id, created_at
		FROM promotion_events
		WHERE workspace_id = $1
		  AND project_id = $2
		  AND ($3::promotion_channel IS NULL OR channel = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`, workspaceID, projectID, channel, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.PromotionEvent, 0)
	eventIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var event domain.PromotionEvent
		if err := rows.Scan(
			&event.ID,
			&event.WorkspaceID,
			&event.ProjectID,
			&event.Channel,
			&event.RunID,
			&event.Note,
			&event.PromotedBy,
			&event.PromotedByPrincipalID,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, event)
		eventIDs = append(eventIDs, event.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	artefactsByEvent, err := s.loadPromotionArtefactsByEventID(ctx, eventIDs)
	if err != nil {
		return nil, err
	}
	for i := range result {
		result[i].Artefacts = artefactsByEvent[result[i].ID]
	}

	return result, nil
}

func (s *Store) ResolvePromotedArtefact(ctx context.Context, workspaceID, projectID uuid.UUID, channel domain.PromotionChannel, logicalKey string) (*domain.PromotedArtefactResolution, error) {
	resolution := &domain.PromotedArtefactResolution{}
	err := s.db.QueryRowContext(ctx, `
		SELECT pcp.channel,
		       pea.logical_key,
		       pe.id,
		       pe.run_id,
		       pea.job_id,
		       pea.path,
		       pea.object_key,
		       pea.size_bytes,
		       COALESCE(pea.content_type, ''),
		       pe.created_at,
		       pe.promoted_by
		FROM project_channel_promotions pcp
		JOIN promotion_events pe ON pe.id = pcp.promotion_event_id
		JOIN promotion_event_artefacts pea ON pea.promotion_event_id = pe.id
		WHERE pe.workspace_id = $1
		  AND pcp.project_id = $2
		  AND pcp.channel = $3
		  AND pea.logical_key = $4
		LIMIT 1
	`, workspaceID, projectID, channel, strings.ToLower(strings.TrimSpace(logicalKey))).Scan(
		&resolution.Channel,
		&resolution.LogicalKey,
		&resolution.PromotionEventID,
		&resolution.RunID,
		&resolution.JobID,
		&resolution.Path,
		&resolution.ObjectKey,
		&resolution.SizeBytes,
		&resolution.ContentType,
		&resolution.PromotedAt,
		&resolution.PromotedBy,
	)
	if err == sql.ErrNoRows {
		return nil, ErrPromotionNotFound
	}
	if err != nil {
		return nil, err
	}
	return resolution, nil
}

func (s *Store) loadPromotionArtefactsByEventID(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID][]domain.PromotionArtefactRef, error) {
	result := make(map[uuid.UUID][]domain.PromotionArtefactRef, len(eventIDs))
	if len(eventIDs) == 0 {
		return result, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT promotion_event_id, logical_key, job_id, path, object_key, size_bytes, COALESCE(content_type, '')
		FROM promotion_event_artefacts
		WHERE promotion_event_id = ANY($1)
		ORDER BY promotion_event_id ASC, logical_key ASC
	`, pq.Array(eventIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var eventID uuid.UUID
		var art domain.PromotionArtefactRef
		if err := rows.Scan(&eventID, &art.LogicalKey, &art.JobID, &art.Path, &art.ObjectKey, &art.SizeBytes, &art.ContentType); err != nil {
			return nil, err
		}
		result[eventID] = append(result[eventID], art)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, eventID := range eventIDs {
		if _, ok := result[eventID]; !ok {
			result[eventID] = []domain.PromotionArtefactRef{}
			continue
		}
		sort.Slice(result[eventID], func(i, j int) bool {
			return result[eventID][i].LogicalKey < result[eventID][j].LogicalKey
		})
	}

	return result, nil
}

func (s *Store) PromotionHistoryCount(ctx context.Context, workspaceID, projectID uuid.UUID, channel *domain.PromotionChannel) (int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM promotion_events
		WHERE workspace_id = $1
		  AND project_id = $2
		  AND ($3::promotion_channel IS NULL OR channel = $3)
	`, workspaceID, projectID, channel).Scan(&total)
	return total, err
}

func PromotionDownloadExpiry(now time.Time, ttl time.Duration) time.Time {
	return now.UTC().Add(ttl)
}
