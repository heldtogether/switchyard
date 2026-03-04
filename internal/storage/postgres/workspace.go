package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

// CreateWorkspace inserts a new workspace
func (s *Store) CreateWorkspace(ctx context.Context, workspace *domain.Workspace) error {
	query := `
		INSERT INTO workspaces (id, slug, name, description, metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at
	`

	metadataJSON, _ := json.Marshal(workspace.Metadata)

	err := s.db.QueryRowContext(ctx, query,
		workspace.ID, workspace.Slug, workspace.Name, workspace.Description, metadataJSON,
	).Scan(&workspace.CreatedAt, &workspace.UpdatedAt)

	return err
}

func (s *Store) EnsureWorkspace(ctx context.Context, slug, name string) (*domain.Workspace, error) {
	ws, err := s.GetWorkspaceBySlug(ctx, slug)
	if err == nil {
		return ws, nil
	}
	workspace := &domain.Workspace{
		ID:   uuid.New(),
		Slug: slug,
		Name: name,
	}
	if err := s.CreateWorkspace(ctx, workspace); err != nil {
		// Handle race where another instance created it.
		return s.GetWorkspaceBySlug(ctx, slug)
	}
	return workspace, nil
}

// GetWorkspace retrieves a workspace by ID
func (s *Store) GetWorkspace(ctx context.Context, id uuid.UUID) (*domain.Workspace, error) {
	query := `
		SELECT id, slug, name, description, created_at, updated_at, metadata
		FROM workspaces WHERE id = $1
	`

	workspace := &domain.Workspace{}
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&workspace.ID, &workspace.Slug, &workspace.Name, &workspace.Description,
		&workspace.CreatedAt, &workspace.UpdatedAt, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workspace not found")
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(metadataJSON, &workspace.Metadata)
	return workspace, nil
}

// GetWorkspaceBySlug retrieves a workspace by slug
func (s *Store) GetWorkspaceBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	query := `
		SELECT id, slug, name, description, created_at, updated_at, metadata
		FROM workspaces WHERE slug = $1
	`

	workspace := &domain.Workspace{}
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, slug).Scan(
		&workspace.ID, &workspace.Slug, &workspace.Name, &workspace.Description,
		&workspace.CreatedAt, &workspace.UpdatedAt, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workspace not found")
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(metadataJSON, &workspace.Metadata)
	return workspace, nil
}

// ListWorkspaces lists all workspaces
func (s *Store) ListWorkspaces(ctx context.Context, limit, offset int) ([]*domain.Workspace, error) {
	query := `
		SELECT id, slug, name, description, created_at, updated_at, metadata
		FROM workspaces
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []*domain.Workspace
	for rows.Next() {
		workspace := &domain.Workspace{}
		var metadataJSON []byte

		err := rows.Scan(
			&workspace.ID, &workspace.Slug, &workspace.Name, &workspace.Description,
			&workspace.CreatedAt, &workspace.UpdatedAt, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(metadataJSON, &workspace.Metadata)
		workspaces = append(workspaces, workspace)
	}

	return workspaces, rows.Err()
}

// UpdateWorkspace updates a workspace
func (s *Store) UpdateWorkspace(ctx context.Context, workspace *domain.Workspace) error {
	query := `
		UPDATE workspaces
		SET name = $1, description = $2, metadata = $3
		WHERE id = $4
	`

	metadataJSON, _ := json.Marshal(workspace.Metadata)

	_, err := s.db.ExecContext(ctx, query,
		workspace.Name, workspace.Description, metadataJSON, workspace.ID,
	)

	return err
}
