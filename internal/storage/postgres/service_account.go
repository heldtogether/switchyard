package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
)

func (s *Store) CreateServiceAccount(ctx context.Context, account *domain.ServiceAccount, principal *domain.Principal, projectIDs []uuid.UUID, workspaceRole, projectRole domain.MemberRole) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if principal.ID == uuid.Nil {
		principal.ID = uuid.New()
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO principals (id, subject, email, display_name, provider)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (subject) DO UPDATE
		SET display_name = EXCLUDED.display_name,
		    provider = EXCLUDED.provider
		RETURNING id, created_at, updated_at
	`, principal.ID, principal.Subject, principal.Email, principal.DisplayName, principal.Provider).Scan(&principal.ID, &principal.CreatedAt, &principal.UpdatedAt); err != nil {
		return err
	}

	if account.ID == uuid.Nil {
		account.ID = uuid.New()
	}
	account.PrincipalID = principal.ID
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO service_accounts (id, workspace_id, principal_id, name, description, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`, account.ID, account.WorkspaceID, account.PrincipalID, account.Name, account.Description, account.CreatedBy).Scan(&account.CreatedAt, &account.UpdatedAt); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workspace_memberships (workspace_id, principal_id, role, created_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, principal_id) DO UPDATE SET role = EXCLUDED.role
	`, account.WorkspaceID, account.PrincipalID, workspaceRole, account.CreatedBy); err != nil {
		return err
	}

	for _, projectID := range projectIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO project_memberships (project_id, principal_id, role, created_by)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (project_id, principal_id) DO UPDATE SET role = EXCLUDED.role
		`, projectID, account.PrincipalID, projectRole, account.CreatedBy); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) CreateServiceAccountKey(ctx context.Context, key *domain.ServiceAccountKey) error {
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	return s.db.QueryRowContext(ctx, `
		INSERT INTO service_account_keys (id, service_account_id, name, token_hash, token_prefix, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`, key.ID, key.ServiceAccountID, key.Name, key.TokenHash, key.TokenPrefix, key.ExpiresAt, key.CreatedBy).Scan(&key.CreatedAt)
}

func (s *Store) ListServiceAccounts(ctx context.Context, workspaceID uuid.UUID) ([]domain.ServiceAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sa.id, sa.workspace_id, sa.principal_id, sa.name, sa.description, sa.disabled_at, sa.disabled_by,
		       sa.created_at, sa.updated_at, sa.created_by,
		       p.subject, p.email, p.display_name, p.provider, p.created_at, p.updated_at
		FROM service_accounts sa
		JOIN principals p ON p.id = sa.principal_id
		WHERE sa.workspace_id = $1
		ORDER BY sa.created_at DESC
	`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ServiceAccount
	for rows.Next() {
		var account domain.ServiceAccount
		var principal domain.Principal
		if err := rows.Scan(
			&account.ID, &account.WorkspaceID, &account.PrincipalID, &account.Name, &account.Description, &account.DisabledAt, &account.DisabledBy,
			&account.CreatedAt, &account.UpdatedAt, &account.CreatedBy,
			&principal.Subject, &principal.Email, &principal.DisplayName, &principal.Provider, &principal.CreatedAt, &principal.UpdatedAt,
		); err != nil {
			return nil, err
		}
		principal.ID = account.PrincipalID
		account.Principal = &principal
		out = append(out, account)
	}
	return out, rows.Err()
}

func (s *Store) GetServiceAccount(ctx context.Context, workspaceID, accountID uuid.UUID) (*domain.ServiceAccount, error) {
	var account domain.ServiceAccount
	var principal domain.Principal
	err := s.db.QueryRowContext(ctx, `
		SELECT sa.id, sa.workspace_id, sa.principal_id, sa.name, sa.description, sa.disabled_at, sa.disabled_by,
		       sa.created_at, sa.updated_at, sa.created_by,
		       p.subject, p.email, p.display_name, p.provider, p.created_at, p.updated_at
		FROM service_accounts sa
		JOIN principals p ON p.id = sa.principal_id
		WHERE sa.workspace_id = $1 AND sa.id = $2
	`, workspaceID, accountID).Scan(
		&account.ID, &account.WorkspaceID, &account.PrincipalID, &account.Name, &account.Description, &account.DisabledAt, &account.DisabledBy,
		&account.CreatedAt, &account.UpdatedAt, &account.CreatedBy,
		&principal.Subject, &principal.Email, &principal.DisplayName, &principal.Provider, &principal.CreatedAt, &principal.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("service account not found")
	}
	if err != nil {
		return nil, err
	}
	principal.ID = account.PrincipalID
	account.Principal = &principal
	return &account, nil
}

func (s *Store) ListServiceAccountKeys(ctx context.Context, serviceAccountID uuid.UUID) ([]domain.ServiceAccountKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, service_account_id, name, token_hash, token_prefix, expires_at, last_used_at, revoked_at, revoked_by, created_at, created_by
		FROM service_account_keys
		WHERE service_account_id = $1
		ORDER BY created_at DESC
	`, serviceAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ServiceAccountKey
	for rows.Next() {
		var key domain.ServiceAccountKey
		if err := rows.Scan(&key.ID, &key.ServiceAccountID, &key.Name, &key.TokenHash, &key.TokenPrefix, &key.ExpiresAt, &key.LastUsedAt, &key.RevokedAt, &key.RevokedBy, &key.CreatedAt, &key.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Store) RevokeServiceAccountKey(ctx context.Context, workspaceID, accountID, keyID uuid.UUID, actor string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE service_account_keys sak
		SET revoked_at = NOW(), revoked_by = $4
		FROM service_accounts sa
		WHERE sak.service_account_id = sa.id
		  AND sa.workspace_id = $1
		  AND sa.id = $2
		  AND sak.id = $3
		  AND sak.revoked_at IS NULL
	`, workspaceID, accountID, keyID, actor)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("service account key not found")
	}
	return nil
}

func (s *Store) DisableServiceAccount(ctx context.Context, workspaceID, accountID uuid.UUID, actor string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE service_accounts
		SET disabled_at = NOW(), disabled_by = $3
		WHERE workspace_id = $1 AND id = $2 AND disabled_at IS NULL
	`, workspaceID, accountID, actor)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("service account not found")
	}
	return nil
}

func (s *Store) ResolveServiceAccountKey(ctx context.Context, tokenHash string) (*domain.ServiceAccount, *domain.ServiceAccountKey, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	var account domain.ServiceAccount
	var key domain.ServiceAccountKey
	var principal domain.Principal
	err = tx.QueryRowContext(ctx, `
		SELECT sa.id, sa.workspace_id, sa.principal_id, sa.name, sa.description, sa.disabled_at, sa.disabled_by,
		       sa.created_at, sa.updated_at, sa.created_by,
		       sak.id, sak.service_account_id, sak.name, sak.token_hash, sak.token_prefix, sak.expires_at, sak.last_used_at, sak.revoked_at, sak.revoked_by, sak.created_at, sak.created_by,
		       p.subject, p.email, p.display_name, p.provider, p.created_at, p.updated_at
		FROM service_account_keys sak
		JOIN service_accounts sa ON sa.id = sak.service_account_id
		JOIN principals p ON p.id = sa.principal_id
		WHERE sak.token_hash = $1
		  AND sak.revoked_at IS NULL
		  AND sak.expires_at > NOW()
		  AND sa.disabled_at IS NULL
		FOR UPDATE OF sak
	`, tokenHash).Scan(
		&account.ID, &account.WorkspaceID, &account.PrincipalID, &account.Name, &account.Description, &account.DisabledAt, &account.DisabledBy,
		&account.CreatedAt, &account.UpdatedAt, &account.CreatedBy,
		&key.ID, &key.ServiceAccountID, &key.Name, &key.TokenHash, &key.TokenPrefix, &key.ExpiresAt, &key.LastUsedAt, &key.RevokedAt, &key.RevokedBy, &key.CreatedAt, &key.CreatedBy,
		&principal.Subject, &principal.Email, &principal.DisplayName, &principal.Provider, &principal.CreatedAt, &principal.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("service account key not found")
	}
	if err != nil {
		return nil, nil, err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE service_account_keys SET last_used_at = $1 WHERE id = $2`, time.Now().UTC(), key.ID); err != nil {
		return nil, nil, err
	}
	principal.ID = account.PrincipalID
	account.Principal = &principal

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return &account, &key, nil
}
