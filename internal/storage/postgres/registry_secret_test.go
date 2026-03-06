package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/stretchr/testify/require"
)

func setupTestWorkspaceForSecrets(t *testing.T, store *Store, ctx context.Context) *domain.Workspace {
	t.Helper()
	workspace := &domain.Workspace{
		ID:   uuid.New(),
		Slug: "secrets-workspace",
		Name: "Secrets Workspace",
	}
	require.NoError(t, store.CreateWorkspace(ctx, workspace))
	return workspace
}

func TestStore_RotateRegistrySecret_CreatesNewActiveAndDeactivatesPrevious(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	workspace := setupTestWorkspaceForSecrets(t, store, ctx)

	original := &domain.RegistrySecret{
		ID:                uuid.New(),
		CreatedBy:         "owner@example.com",
		WorkspaceID:       workspace.ID,
		Host:              "docker.io",
		Username:          "svc-user",
		PasswordEncrypted: "old-secret",
		Active:            true,
	}
	require.NoError(t, store.CreateRegistrySecret(ctx, original))

	rotated, err := store.RotateRegistrySecret(ctx, workspace.ID, original.ID, "new-secret", "owner@example.com")
	require.NoError(t, err)
	require.NotEqual(t, original.ID, rotated.ID)
	require.Equal(t, original.Host, rotated.Host)
	require.Equal(t, original.Username, rotated.Username)
	require.True(t, rotated.Active)
	require.NotNil(t, rotated.RotatedFromID)
	require.Equal(t, original.ID, *rotated.RotatedFromID)

	originalAfter, err := store.GetRegistrySecret(ctx, original.ID)
	require.NoError(t, err)
	require.False(t, originalAfter.Active)
	require.NotNil(t, originalAfter.DeactivatedAt)
	require.NotNil(t, originalAfter.DeactivatedBy)
	require.Equal(t, "owner@example.com", *originalAfter.DeactivatedBy)

	active, err := store.GetActiveRegistrySecretForWorkspace(ctx, workspace.ID, rotated.ID)
	require.NoError(t, err)
	require.Equal(t, rotated.ID, active.ID)

	_, err = store.GetActiveRegistrySecretForWorkspace(ctx, workspace.ID, original.ID)
	require.Error(t, err)
}

func TestStore_DeactivateRegistrySecret_MarksSecretInactive(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	workspace := setupTestWorkspaceForSecrets(t, store, ctx)

	secret := &domain.RegistrySecret{
		ID:                uuid.New(),
		CreatedBy:         "owner@example.com",
		WorkspaceID:       workspace.ID,
		Host:              "ghcr.io",
		Username:          "bot",
		PasswordEncrypted: "token",
		Active:            true,
	}
	require.NoError(t, store.CreateRegistrySecret(ctx, secret))

	require.NoError(t, store.DeactivateRegistrySecret(ctx, workspace.ID, secret.ID, "owner@example.com"))

	after, err := store.GetRegistrySecret(ctx, secret.ID)
	require.NoError(t, err)
	require.False(t, after.Active)
	require.NotNil(t, after.DeactivatedAt)
	require.NotNil(t, after.DeactivatedBy)
	require.Equal(t, "owner@example.com", *after.DeactivatedBy)
}
