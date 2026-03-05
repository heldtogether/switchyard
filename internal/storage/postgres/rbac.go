package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

func (s *Store) UpsertPrincipal(ctx context.Context, principal *domain.Principal) error {
	query := `
		INSERT INTO principals (id, subject, email, display_name, provider)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (subject) DO UPDATE
		SET email = EXCLUDED.email,
		    display_name = EXCLUDED.display_name,
		    provider = EXCLUDED.provider
		RETURNING id, created_at, updated_at
	`
	if principal.ID == uuid.Nil {
		principal.ID = uuid.New()
	}
	return s.db.QueryRowContext(ctx, query,
		principal.ID, principal.Subject, principal.Email, principal.DisplayName, principal.Provider,
	).Scan(&principal.ID, &principal.CreatedAt, &principal.UpdatedAt)
}

func (s *Store) GetPrincipalBySubject(ctx context.Context, subject string) (*domain.Principal, error) {
	query := `
		SELECT id, subject, email, display_name, provider, created_at, updated_at
		FROM principals
		WHERE subject = $1
	`
	var p domain.Principal
	err := s.db.QueryRowContext(ctx, query, subject).Scan(
		&p.ID, &p.Subject, &p.Email, &p.DisplayName, &p.Provider, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("principal not found")
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) CreateWorkspaceMembership(ctx context.Context, m *domain.WorkspaceMembership) error {
	query := `
		INSERT INTO workspace_memberships (workspace_id, principal_id, role, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, principal_id) DO UPDATE SET role = EXCLUDED.role
		RETURNING created_at
	`
	return s.db.QueryRowContext(ctx, query, m.WorkspaceID, m.PrincipalID, m.Role, m.CreatedBy).Scan(&m.CreatedAt)
}

func (s *Store) CreateProjectMembership(ctx context.Context, m *domain.ProjectMembership) error {
	query := `
		INSERT INTO project_memberships (project_id, principal_id, role, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, principal_id) DO UPDATE SET role = EXCLUDED.role
		RETURNING created_at
	`
	return s.db.QueryRowContext(ctx, query, m.ProjectID, m.PrincipalID, m.Role, m.CreatedBy).Scan(&m.CreatedAt)
}

func (s *Store) WorkspaceRoleForPrincipal(ctx context.Context, workspaceID, principalID uuid.UUID) (*domain.MemberRole, error) {
	query := `SELECT role FROM workspace_memberships WHERE workspace_id = $1 AND principal_id = $2`
	var role domain.MemberRole
	err := s.db.QueryRowContext(ctx, query, workspaceID, principalID).Scan(&role)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *Store) ProjectRoleForPrincipal(ctx context.Context, projectID, principalID uuid.UUID) (*domain.MemberRole, error) {
	query := `SELECT role FROM project_memberships WHERE project_id = $1 AND principal_id = $2`
	var role domain.MemberRole
	err := s.db.QueryRowContext(ctx, query, projectID, principalID).Scan(&role)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (s *Store) CountWorkspaceOwners(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM workspace_memberships WHERE workspace_id = $1 AND role = 'owner'`
	var count int
	err := s.db.QueryRowContext(ctx, query, workspaceID).Scan(&count)
	return count, err
}

func (s *Store) ListWorkspaceMembers(ctx context.Context, workspaceID uuid.UUID) ([]domain.WorkspaceMembership, error) {
	query := `
		SELECT wm.workspace_id, wm.principal_id, wm.role, wm.created_at, wm.created_by,
		       p.subject, p.email, p.display_name, p.provider, p.created_at, p.updated_at
		FROM workspace_memberships wm
		JOIN principals p ON p.id = wm.principal_id
		WHERE wm.workspace_id = $1
		ORDER BY wm.created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WorkspaceMembership
	for rows.Next() {
		var m domain.WorkspaceMembership
		var p domain.Principal
		if err := rows.Scan(
			&m.WorkspaceID, &m.PrincipalID, &m.Role, &m.CreatedAt, &m.CreatedBy,
			&p.Subject, &p.Email, &p.DisplayName, &p.Provider, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.ID = m.PrincipalID
		m.Principal = &p
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) ListWorkspaceMembershipsForPrincipal(ctx context.Context, principalID uuid.UUID) ([]domain.WorkspaceMembership, error) {
	query := `
		SELECT wm.workspace_id, wm.principal_id, wm.role, wm.created_at, wm.created_by,
		       w.slug, w.name, w.description, w.created_at, w.updated_at, w.metadata
		FROM workspace_memberships wm
		JOIN workspaces w ON w.id = wm.workspace_id
		WHERE wm.principal_id = $1
		ORDER BY w.created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, principalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.WorkspaceMembership
	for rows.Next() {
		var m domain.WorkspaceMembership
		var w domain.Workspace
		var metadata []byte
		if err := rows.Scan(
			&m.WorkspaceID, &m.PrincipalID, &m.Role, &m.CreatedAt, &m.CreatedBy,
			&w.Slug, &w.Name, &w.Description, &w.CreatedAt, &w.UpdatedAt, &metadata,
		); err != nil {
			return nil, err
		}
		w.ID = m.WorkspaceID
		_ = json.Unmarshal(metadata, &w.Metadata)
		m.Workspace = &w
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) ListProjectMembershipsForPrincipal(ctx context.Context, principalID uuid.UUID) ([]domain.ProjectMembership, error) {
	query := `
		SELECT pm.project_id, pm.principal_id, pm.role, pm.created_at, pm.created_by,
		       p.workspace_id, p.slug, p.name, p.description, p.created_at, p.updated_at, p.created_by, p.archived, p.metadata
		FROM project_memberships pm
		JOIN projects p ON p.id = pm.project_id
		WHERE pm.principal_id = $1
		ORDER BY p.created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, principalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ProjectMembership
	for rows.Next() {
		var m domain.ProjectMembership
		var p domain.Project
		var metadata []byte
		if err := rows.Scan(
			&m.ProjectID, &m.PrincipalID, &m.Role, &m.CreatedAt, &m.CreatedBy,
			&p.WorkspaceID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.CreatedBy, &p.Archived, &metadata,
		); err != nil {
			return nil, err
		}
		p.ID = m.ProjectID
		_ = json.Unmarshal(metadata, &p.Metadata)
		m.Project = &p
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]domain.ProjectMembership, error) {
	query := `
		SELECT pm.project_id, pm.principal_id, pm.role, pm.created_at, pm.created_by,
		       p.subject, p.email, p.display_name, p.provider, p.created_at, p.updated_at
		FROM project_memberships pm
		JOIN principals p ON p.id = pm.principal_id
		WHERE pm.project_id = $1
		ORDER BY pm.created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ProjectMembership
	for rows.Next() {
		var m domain.ProjectMembership
		var p domain.Principal
		if err := rows.Scan(
			&m.ProjectID, &m.PrincipalID, &m.Role, &m.CreatedAt, &m.CreatedBy,
			&p.Subject, &p.Email, &p.DisplayName, &p.Provider, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.ID = m.PrincipalID
		m.Principal = &p
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CreateWorkspaceInvite(ctx context.Context, inv *domain.WorkspaceInvite) error {
	query := `
		INSERT INTO workspace_invites (id, workspace_id, email, role, token_hash, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	inv.Email = strings.ToLower(strings.TrimSpace(inv.Email))
	return s.db.QueryRowContext(ctx, query, inv.ID, inv.WorkspaceID, inv.Email, inv.Role, inv.TokenHash, inv.ExpiresAt, inv.CreatedBy).Scan(&inv.CreatedAt)
}

func (s *Store) CreateProjectInvite(ctx context.Context, inv *domain.ProjectInvite) error {
	query := `
		INSERT INTO project_invites (id, project_id, email, role, token_hash, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	inv.Email = strings.ToLower(strings.TrimSpace(inv.Email))
	return s.db.QueryRowContext(ctx, query, inv.ID, inv.ProjectID, inv.Email, inv.Role, inv.TokenHash, inv.ExpiresAt, inv.CreatedBy).Scan(&inv.CreatedAt)
}

func (s *Store) AcceptWorkspaceInvite(ctx context.Context, tokenHash string, principalID uuid.UUID, principalEmail, actor string) (*domain.WorkspaceMembership, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var invite domain.WorkspaceInvite
	err = tx.QueryRowContext(ctx, `
		SELECT id, workspace_id, email, role, token_hash, expires_at, accepted_at, created_at, created_by
		FROM workspace_invites
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash).Scan(
		&invite.ID, &invite.WorkspaceID, &invite.Email, &invite.Role, &invite.TokenHash, &invite.ExpiresAt, &invite.AcceptedAt, &invite.CreatedAt, &invite.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invite not found")
	}
	if err != nil {
		return nil, err
	}
	if invite.AcceptedAt != nil {
		return nil, fmt.Errorf("invite already accepted")
	}
	if time.Now().After(invite.ExpiresAt) {
		return nil, fmt.Errorf("invite expired")
	}
	if !strings.EqualFold(invite.Email, strings.TrimSpace(principalEmail)) {
		return nil, fmt.Errorf("invite email mismatch")
	}

	m := &domain.WorkspaceMembership{
		WorkspaceID: invite.WorkspaceID,
		PrincipalID: principalID,
		Role:        invite.Role,
		CreatedBy:   actor,
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO workspace_memberships (workspace_id, principal_id, role, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, principal_id) DO UPDATE SET role = EXCLUDED.role
		RETURNING created_at
	`, m.WorkspaceID, m.PrincipalID, m.Role, m.CreatedBy).Scan(&m.CreatedAt); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE workspace_invites SET accepted_at = NOW() WHERE id = $1`, invite.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) AcceptProjectInvite(ctx context.Context, tokenHash string, principalID uuid.UUID, principalEmail, actor string) (*domain.ProjectMembership, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var invite domain.ProjectInvite
	err = tx.QueryRowContext(ctx, `
		SELECT id, project_id, email, role, token_hash, expires_at, accepted_at, created_at, created_by
		FROM project_invites
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash).Scan(
		&invite.ID, &invite.ProjectID, &invite.Email, &invite.Role, &invite.TokenHash, &invite.ExpiresAt, &invite.AcceptedAt, &invite.CreatedAt, &invite.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invite not found")
	}
	if err != nil {
		return nil, err
	}
	if invite.AcceptedAt != nil {
		return nil, fmt.Errorf("invite already accepted")
	}
	if time.Now().After(invite.ExpiresAt) {
		return nil, fmt.Errorf("invite expired")
	}
	if !strings.EqualFold(invite.Email, strings.TrimSpace(principalEmail)) {
		return nil, fmt.Errorf("invite email mismatch")
	}

	var workspaceID uuid.UUID
	if err := tx.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id = $1`, invite.ProjectID).Scan(&workspaceID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("project not found")
		}
		return nil, err
	}

	// Project access requires workspace access, so ensure a baseline workspace membership exists.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspace_memberships (workspace_id, principal_id, role, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, principal_id) DO NOTHING
	`, workspaceID, principalID, domain.MemberRoleMember, actor); err != nil {
		return nil, err
	}
	var workspaceRole domain.MemberRole
	if err := tx.QueryRowContext(ctx, `
		SELECT role
		FROM workspace_memberships
		WHERE workspace_id = $1 AND principal_id = $2
	`, workspaceID, principalID).Scan(&workspaceRole); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workspace membership was not created")
		}
		return nil, err
	}

	m := &domain.ProjectMembership{
		ProjectID:   invite.ProjectID,
		PrincipalID: principalID,
		Role:        invite.Role,
		CreatedBy:   actor,
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO project_memberships (project_id, principal_id, role, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, principal_id) DO UPDATE SET role = EXCLUDED.role
		RETURNING created_at
	`, m.ProjectID, m.PrincipalID, m.Role, m.CreatedBy).Scan(&m.CreatedAt); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE project_invites SET accepted_at = NOW() WHERE id = $1`, invite.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return m, nil
}
