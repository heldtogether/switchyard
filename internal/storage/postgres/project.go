package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// CreateProject inserts a new project
func (s *Store) CreateProject(ctx context.Context, project *domain.Project) error {
	query := `
		INSERT INTO projects (id, workspace_id, slug, name, description, created_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`

	metadataJSON, _ := json.Marshal(project.Metadata)

	err := s.db.QueryRowContext(ctx, query,
		project.ID, project.WorkspaceID, project.Slug, project.Name,
		project.Description, project.CreatedBy, metadataJSON,
	).Scan(&project.CreatedAt, &project.UpdatedAt)

	return err
}

// GetProject retrieves a project by ID
func (s *Store) GetProject(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	query := `
		SELECT id, workspace_id, slug, name, description, created_at, updated_at, 
		       created_by, archived, metadata
		FROM projects WHERE id = $1
	`

	project := &domain.Project{}
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&project.ID, &project.WorkspaceID, &project.Slug, &project.Name,
		&project.Description, &project.CreatedAt, &project.UpdatedAt,
		&project.CreatedBy, &project.Archived, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found")
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(metadataJSON, &project.Metadata)
	return project, nil
}

// GetProjectBySlug retrieves a project by workspace ID and slug
func (s *Store) GetProjectBySlug(ctx context.Context, workspaceID uuid.UUID, slug string) (*domain.Project, error) {
	query := `
		SELECT id, workspace_id, slug, name, description, created_at, updated_at, 
		       created_by, archived, metadata
		FROM projects WHERE workspace_id = $1 AND slug = $2
	`

	project := &domain.Project{}
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, workspaceID, slug).Scan(
		&project.ID, &project.WorkspaceID, &project.Slug, &project.Name,
		&project.Description, &project.CreatedAt, &project.UpdatedAt,
		&project.CreatedBy, &project.Archived, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found")
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(metadataJSON, &project.Metadata)
	return project, nil
}

// ListProjects lists projects for a workspace
func (s *Store) ListProjects(ctx context.Context, workspaceID uuid.UUID, includeArchived bool, limit, offset int) ([]*domain.Project, error) {
	query := `
		SELECT id, workspace_id, slug, name, description, created_at, updated_at, 
		       created_by, archived, metadata
		FROM projects
		WHERE workspace_id = $1
		  AND ($2 = true OR archived = false)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := s.db.QueryContext(ctx, query, workspaceID, includeArchived, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		project := &domain.Project{}
		var metadataJSON []byte

		err := rows.Scan(
			&project.ID, &project.WorkspaceID, &project.Slug, &project.Name,
			&project.Description, &project.CreatedAt, &project.UpdatedAt,
			&project.CreatedBy, &project.Archived, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(metadataJSON, &project.Metadata)
		projects = append(projects, project)
	}

	return projects, rows.Err()
}

// UpdateProject updates a project
func (s *Store) UpdateProject(ctx context.Context, project *domain.Project) error {
	query := `
		UPDATE projects
		SET name = $1, description = $2, metadata = $3, archived = $4
		WHERE id = $5
	`

	metadataJSON, _ := json.Marshal(project.Metadata)

	_, err := s.db.ExecContext(ctx, query,
		project.Name, project.Description, metadataJSON, project.Archived, project.ID,
	)

	return err
}

// ArchiveProject archives a project
func (s *Store) ArchiveProject(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE projects SET archived = true WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// UnarchiveProject unarchives a project
func (s *Store) UnarchiveProject(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE projects SET archived = false WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}
