package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestStore_ServiceAccountKeyLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	workspace := &domain.Workspace{ID: uuid.New(), Slug: "sa-workspace", Name: "Service Account Workspace"}
	require.NoError(t, store.CreateWorkspace(ctx, workspace))
	project := &domain.Project{ID: uuid.New(), WorkspaceID: workspace.ID, Slug: "ci", Name: "CI", CreatedBy: "owner"}
	require.NoError(t, store.CreateProject(ctx, project))

	accountID := uuid.New()
	displayName := "github-actions"
	provider := "service_account"
	principal := &domain.Principal{
		ID:          uuid.New(),
		Subject:     "service_account:" + accountID.String(),
		DisplayName: &displayName,
		Provider:    &provider,
	}
	account := &domain.ServiceAccount{
		ID:          accountID,
		WorkspaceID: workspace.ID,
		Name:        "github-actions",
		CreatedBy:   "owner@example.com",
	}

	require.NoError(t, store.CreateServiceAccount(ctx, account, principal, []uuid.UUID{project.ID}, domain.MemberRoleMember, domain.MemberRoleMember))
	require.Equal(t, principal.ID, account.PrincipalID)

	workspaceRole, err := store.WorkspaceRoleForPrincipal(ctx, workspace.ID, principal.ID)
	require.NoError(t, err)
	require.NotNil(t, workspaceRole)
	require.Equal(t, domain.MemberRoleMember, *workspaceRole)

	projectRole, err := store.ProjectRoleForPrincipal(ctx, project.ID, principal.ID)
	require.NoError(t, err)
	require.NotNil(t, projectRole)
	require.Equal(t, domain.MemberRoleMember, *projectRole)

	key := &domain.ServiceAccountKey{
		ID:               uuid.New(),
		ServiceAccountID: account.ID,
		TokenHash:        "hash-1",
		TokenPrefix:      "swy_sa_hash",
		ExpiresAt:        time.Now().Add(time.Hour),
		CreatedBy:        "owner@example.com",
	}
	require.NoError(t, store.CreateServiceAccountKey(ctx, key))

	resolvedAccount, resolvedKey, err := store.ResolveServiceAccountKey(ctx, key.TokenHash)
	require.NoError(t, err)
	require.Equal(t, account.ID, resolvedAccount.ID)
	require.Equal(t, key.ID, resolvedKey.ID)
	require.NotNil(t, resolvedAccount.Principal)
	require.Equal(t, principal.Subject, resolvedAccount.Principal.Subject)

	keys, err := store.ListServiceAccountKeys(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.NotNil(t, keys[0].LastUsedAt)

	require.NoError(t, store.RevokeServiceAccountKey(ctx, workspace.ID, account.ID, key.ID, "owner@example.com"))
	_, _, err = store.ResolveServiceAccountKey(ctx, key.TokenHash)
	require.Error(t, err)
}

func TestStore_ServiceAccountDisabledRejectsKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	workspace := &domain.Workspace{ID: uuid.New(), Slug: "sa-disabled", Name: "Disabled SA Workspace"}
	require.NoError(t, store.CreateWorkspace(ctx, workspace))

	accountID := uuid.New()
	provider := "service_account"
	principal := &domain.Principal{
		ID:       uuid.New(),
		Subject:  "service_account:" + accountID.String(),
		Provider: &provider,
	}
	account := &domain.ServiceAccount{
		ID:          accountID,
		WorkspaceID: workspace.ID,
		Name:        "disabled-ci",
		CreatedBy:   "owner@example.com",
	}
	require.NoError(t, store.CreateServiceAccount(ctx, account, principal, nil, domain.MemberRoleMember, domain.MemberRoleMember))

	key := &domain.ServiceAccountKey{
		ID:               uuid.New(),
		ServiceAccountID: account.ID,
		TokenHash:        "hash-disabled",
		TokenPrefix:      "swy_sa_disable",
		ExpiresAt:        time.Now().Add(time.Hour),
		CreatedBy:        "owner@example.com",
	}
	require.NoError(t, store.CreateServiceAccountKey(ctx, key))
	require.NoError(t, store.DisableServiceAccount(ctx, workspace.ID, account.ID, "owner@example.com"))

	_, _, err := store.ResolveServiceAccountKey(ctx, key.TokenHash)
	require.Error(t, err)
}
