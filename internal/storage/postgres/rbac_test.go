package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestStore_AcceptProjectInvite_CreatesProjectMembershipOnly(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	workspace := &domain.Workspace{
		ID:   uuid.New(),
		Slug: "ws-rbac-test",
		Name: "RBAC Workspace",
	}
	require.NoError(t, store.CreateWorkspace(ctx, workspace))

	project := &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        "proj-rbac-test",
		Name:        "RBAC Project",
		CreatedBy:   "test",
	}
	require.NoError(t, store.CreateProject(ctx, project))

	principal := &domain.Principal{
		Subject: "oidc|invitee",
		Email:   ptrString("invitee@example.com"),
	}
	require.NoError(t, store.UpsertPrincipal(ctx, principal))

	invite := &domain.ProjectInvite{
		ProjectID: project.ID,
		Email:     "invitee@example.com",
		Role:      domain.MemberRoleMember,
		TokenHash: "token-hash-project-membership",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedBy: "owner@example.com",
	}
	require.NoError(t, store.CreateProjectInvite(ctx, invite))

	_, err := store.AcceptProjectInvite(ctx, invite.TokenHash, principal.ID, "invitee@example.com", "owner@example.com")
	require.NoError(t, err)

	projectRole, err := store.ProjectRoleForPrincipal(ctx, project.ID, principal.ID)
	require.NoError(t, err)
	require.NotNil(t, projectRole)
	require.Equal(t, domain.MemberRoleMember, *projectRole)

	workspaceRole, err := store.WorkspaceRoleForPrincipal(ctx, workspace.ID, principal.ID)
	require.NoError(t, err)
	require.Nil(t, workspaceRole)
}

func TestStore_AcceptProjectInvite_DoesNotDowngradeExistingWorkspaceRole(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	workspace := &domain.Workspace{
		ID:   uuid.New(),
		Slug: "ws-owner-keep",
		Name: "Owner Keep",
	}
	require.NoError(t, store.CreateWorkspace(ctx, workspace))

	project := &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        "proj-owner-keep",
		Name:        "Project Owner Keep",
		CreatedBy:   "test",
	}
	require.NoError(t, store.CreateProject(ctx, project))

	principal := &domain.Principal{
		Subject: "oidc|existing-owner",
		Email:   ptrString("owner@example.com"),
	}
	require.NoError(t, store.UpsertPrincipal(ctx, principal))
	require.NoError(t, store.CreateWorkspaceMembership(ctx, &domain.WorkspaceMembership{
		WorkspaceID: workspace.ID,
		PrincipalID: principal.ID,
		Role:        domain.MemberRoleOwner,
		CreatedBy:   "bootstrap",
	}))

	invite := &domain.ProjectInvite{
		ProjectID: project.ID,
		Email:     "owner@example.com",
		Role:      domain.MemberRoleMember,
		TokenHash: "token-hash-owner-keep",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedBy: "owner@example.com",
	}
	require.NoError(t, store.CreateProjectInvite(ctx, invite))

	_, err := store.AcceptProjectInvite(ctx, invite.TokenHash, principal.ID, "owner@example.com", "owner@example.com")
	require.NoError(t, err)

	workspaceRole, err := store.WorkspaceRoleForPrincipal(ctx, workspace.ID, principal.ID)
	require.NoError(t, err)
	require.NotNil(t, workspaceRole)
	require.Equal(t, domain.MemberRoleOwner, *workspaceRole)
}

func ptrString(v string) *string {
	return &v
}
