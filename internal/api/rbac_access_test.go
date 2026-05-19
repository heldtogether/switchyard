package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/stretchr/testify/require"
)

func testRBACAPI(store *postgres.Store) *API {
	return &API{
		cfg: &config.Config{
			API: config.APIConfig{
				RBAC: config.RBACConfig{Enabled: true},
			},
		},
		store:  store,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func requestWithPrincipal(method, target string, body *bytes.Buffer, principal Principal) *http.Request {
	if body == nil {
		body = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, target, body)
	return req.WithContext(context.WithValue(req.Context(), principalContextKey{}, principal))
}

func createRBACPrincipal(t *testing.T, store *postgres.Store, subject, email string) *domain.Principal {
	t.Helper()
	principal := &domain.Principal{Subject: subject, Email: &email}
	require.NoError(t, store.UpsertPrincipal(context.Background(), principal))
	return principal
}

func ptrString(v string) *string {
	return &v
}

func createRBACWorkspaceProject(t *testing.T, store *postgres.Store, workspaceSlug, projectSlug string) (*domain.Workspace, *domain.Project) {
	t.Helper()
	ctx := context.Background()
	workspace := &domain.Workspace{ID: uuid.New(), Slug: workspaceSlug, Name: workspaceSlug}
	require.NoError(t, store.CreateWorkspace(ctx, workspace))
	project := &domain.Project{ID: uuid.New(), WorkspaceID: workspace.ID, Slug: projectSlug, Name: projectSlug, CreatedBy: "test"}
	require.NoError(t, store.CreateProject(ctx, project))
	return workspace, project
}

func TestRBACWorkspaceOwnerCanListAllProjects(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	workspace, _ := createRBACWorkspaceProject(t, store, "owner-ws", "project-a")
	projectB := &domain.Project{ID: uuid.New(), WorkspaceID: workspace.ID, Slug: "project-b", Name: "Project B", CreatedBy: "test"}
	require.NoError(t, store.CreateProject(ctx, projectB))
	owner := createRBACPrincipal(t, store, "oidc|owner-list", "owner-list@example.com")
	require.NoError(t, store.CreateWorkspaceMembership(ctx, &domain.WorkspaceMembership{
		WorkspaceID: workspace.ID,
		PrincipalID: owner.ID,
		Role:        domain.MemberRoleOwner,
		CreatedBy:   "test",
	}))

	api := testRBACAPI(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects", api.HandleListProjects)

	req := requestWithPrincipal(http.MethodGet, "/v1/workspaces/owner-ws/projects", nil, Principal{Subject: owner.Subject, Email: "owner-list@example.com", AuthMethod: "oidc"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response struct {
		Projects []ProjectResponse `json:"projects"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Projects, 2)
}

func TestRBACWorkspaceOwnerCreateThenListShowsCreatedProject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	workspace, _ := createRBACWorkspaceProject(t, store, "owner-create-list-ws", "existing")
	owner := createRBACPrincipal(t, store, "oidc|owner-create-list", "owner-create-list@example.com")
	require.NoError(t, store.CreateWorkspaceMembership(ctx, &domain.WorkspaceMembership{
		WorkspaceID: workspace.ID,
		PrincipalID: owner.ID,
		Role:        domain.MemberRoleOwner,
		CreatedBy:   "test",
	}))

	api := testRBACAPI(store)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects", api.HandleCreateProject)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects", api.HandleListProjects)

	principal := Principal{Subject: owner.Subject, Email: "owner-create-list@example.com", AuthMethod: "oidc"}
	body := bytes.NewBufferString(`{"slug":"created","name":"Created"}`)
	createProject := requestWithPrincipal(http.MethodPost, "/v1/workspaces/owner-create-list-ws/projects", body, principal)
	createRecorder := httptest.NewRecorder()
	mux.ServeHTTP(createRecorder, createProject)
	require.Equal(t, http.StatusCreated, createRecorder.Code)

	listProjects := requestWithPrincipal(http.MethodGet, "/v1/workspaces/owner-create-list-ws/projects", nil, principal)
	listRecorder := httptest.NewRecorder()
	mux.ServeHTTP(listRecorder, listProjects)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	var response struct {
		Projects []ProjectResponse `json:"projects"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &response))
	require.Len(t, response.Projects, 2)
	slugs := make([]string, 0, len(response.Projects))
	for _, project := range response.Projects {
		slugs = append(slugs, project.Slug)
	}
	require.Contains(t, slugs, "existing")
	require.Contains(t, slugs, "created")
}

func TestRBACWorkspaceMemberCanListAllProjects(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	workspace, projectA := createRBACWorkspaceProject(t, store, "member-list-ws", "project-a")
	projectB := &domain.Project{ID: uuid.New(), WorkspaceID: workspace.ID, Slug: "project-b", Name: "Project B", CreatedBy: "test"}
	require.NoError(t, store.CreateProject(ctx, projectB))
	member := createRBACPrincipal(t, store, "oidc|member-list", "member-list@example.com")
	require.NoError(t, store.CreateWorkspaceMembership(ctx, &domain.WorkspaceMembership{
		WorkspaceID: workspace.ID,
		PrincipalID: member.ID,
		Role:        domain.MemberRoleMember,
		CreatedBy:   "test",
	}))

	api := testRBACAPI(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects", api.HandleListProjects)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}", api.HandleGetProject)

	principal := Principal{Subject: member.Subject, Email: "member-list@example.com", AuthMethod: "oidc"}
	listProjects := requestWithPrincipal(http.MethodGet, "/v1/workspaces/member-list-ws/projects", nil, principal)
	listRecorder := httptest.NewRecorder()
	mux.ServeHTTP(listRecorder, listProjects)

	require.Equal(t, http.StatusOK, listRecorder.Code)
	var listResponse struct {
		Projects []ProjectResponse `json:"projects"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listResponse))
	require.Len(t, listResponse.Projects, 2)

	getProject := requestWithPrincipal(http.MethodGet, "/v1/workspaces/member-list-ws/projects/"+projectA.Slug, nil, principal)
	getRecorder := httptest.NewRecorder()
	mux.ServeHTTP(getRecorder, getProject)
	require.Equal(t, http.StatusOK, getRecorder.Code)
}

func TestRBACProjectOnlyUserCanAccessProjectWithoutWorkspaceMembership(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	workspace, project := createRBACWorkspaceProject(t, store, "project-only-ws", "project-only")
	siblingProject := &domain.Project{ID: uuid.New(), WorkspaceID: workspace.ID, Slug: "sibling", Name: "Sibling", CreatedBy: "test"}
	require.NoError(t, store.CreateProject(ctx, siblingProject))
	member := createRBACPrincipal(t, store, "oidc|project-only", "project-only@example.com")
	require.NoError(t, store.CreateProjectMembership(ctx, &domain.ProjectMembership{
		ProjectID:   project.ID,
		PrincipalID: member.ID,
		Role:        domain.MemberRoleMember,
		CreatedBy:   "test",
	}))

	api := testRBACAPI(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/workspaces", api.HandleListWorkspaces)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects", api.HandleListProjects)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}", api.HandleGetProject)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/members", api.HandleListWorkspaceMembers)

	principal := Principal{Subject: member.Subject, Email: "project-only@example.com", AuthMethod: "oidc"}
	listWorkspaces := requestWithPrincipal(http.MethodGet, "/v1/workspaces", nil, principal)
	workspacesRecorder := httptest.NewRecorder()
	mux.ServeHTTP(workspacesRecorder, listWorkspaces)
	require.Equal(t, http.StatusOK, workspacesRecorder.Code)
	var workspacesResponse struct {
		Workspaces []WorkspaceResponse `json:"workspaces"`
	}
	require.NoError(t, json.Unmarshal(workspacesRecorder.Body.Bytes(), &workspacesResponse))
	require.Len(t, workspacesResponse.Workspaces, 1)
	require.Equal(t, "project", workspacesResponse.Workspaces[0].AccessSource)

	listProjects := requestWithPrincipal(http.MethodGet, "/v1/workspaces/project-only-ws/projects", nil, principal)
	projectsRecorder := httptest.NewRecorder()
	mux.ServeHTTP(projectsRecorder, listProjects)
	require.Equal(t, http.StatusOK, projectsRecorder.Code)
	var projectsResponse struct {
		Projects []ProjectResponse `json:"projects"`
	}
	require.NoError(t, json.Unmarshal(projectsRecorder.Body.Bytes(), &projectsResponse))
	require.Len(t, projectsResponse.Projects, 1)
	require.Equal(t, "project-only", projectsResponse.Projects[0].Slug)

	getProject := requestWithPrincipal(http.MethodGet, "/v1/workspaces/project-only-ws/projects/project-only", nil, principal)
	projectRecorder := httptest.NewRecorder()
	mux.ServeHTTP(projectRecorder, getProject)
	require.Equal(t, http.StatusOK, projectRecorder.Code)

	getSiblingProject := requestWithPrincipal(http.MethodGet, "/v1/workspaces/project-only-ws/projects/sibling", nil, principal)
	siblingRecorder := httptest.NewRecorder()
	mux.ServeHTTP(siblingRecorder, getSiblingProject)
	require.Equal(t, http.StatusForbidden, siblingRecorder.Code)

	listWorkspaceMembers := requestWithPrincipal(http.MethodGet, "/v1/workspaces/project-only-ws/members", nil, principal)
	workspaceRecorder := httptest.NewRecorder()
	mux.ServeHTTP(workspaceRecorder, listWorkspaceMembers)
	require.Equal(t, http.StatusForbidden, workspaceRecorder.Code)
}

func TestRBACListWorkspaceMembersExcludesServiceAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	workspace := &domain.Workspace{ID: uuid.New(), Slug: "members-ws", Name: "Members"}
	require.NoError(t, store.CreateWorkspace(ctx, workspace))
	owner := createRBACPrincipal(t, store, "oidc|members-owner", "members-owner@example.com")
	require.NoError(t, store.CreateWorkspaceMembership(ctx, &domain.WorkspaceMembership{
		WorkspaceID: workspace.ID,
		PrincipalID: owner.ID,
		Role:        domain.MemberRoleOwner,
		CreatedBy:   "test",
	}))
	provider := "service_account"
	serviceAccount := &domain.Principal{
		Subject:     "service_account:" + uuid.NewString(),
		DisplayName: ptrString("deploy-bot"),
		Provider:    &provider,
	}
	require.NoError(t, store.UpsertPrincipal(ctx, serviceAccount))
	require.NoError(t, store.CreateWorkspaceMembership(ctx, &domain.WorkspaceMembership{
		WorkspaceID: workspace.ID,
		PrincipalID: serviceAccount.ID,
		Role:        domain.MemberRoleMember,
		CreatedBy:   "test",
	}))

	api := testRBACAPI(store)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/members", api.HandleListWorkspaceMembers)

	req := requestWithPrincipal(http.MethodGet, "/v1/workspaces/members-ws/members", nil, Principal{Subject: owner.Subject, Email: "members-owner@example.com", AuthMethod: "oidc"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var response struct {
		Members []MemberResponse `json:"members"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Members, 1)
	require.Equal(t, owner.Subject, response.Members[0].Subject)
}

func TestRBACWorkspaceMemberCanCreateProject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	workspace := &domain.Workspace{ID: uuid.New(), Slug: "member-create-ws", Name: "Member Create"}
	require.NoError(t, store.CreateWorkspace(ctx, workspace))
	member := createRBACPrincipal(t, store, "oidc|workspace-member", "workspace-member@example.com")
	require.NoError(t, store.CreateWorkspaceMembership(ctx, &domain.WorkspaceMembership{
		WorkspaceID: workspace.ID,
		PrincipalID: member.ID,
		Role:        domain.MemberRoleMember,
		CreatedBy:   "test",
	}))

	api := testRBACAPI(store)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects", api.HandleCreateProject)

	body := bytes.NewBufferString(`{"slug":"created-by-member","name":"Created By Member"}`)
	req := requestWithPrincipal(http.MethodPost, "/v1/workspaces/member-create-ws/projects", body, Principal{Subject: member.Subject, Email: "workspace-member@example.com", AuthMethod: "oidc"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	project, err := store.GetProjectBySlug(ctx, workspace.ID, "created-by-member")
	require.NoError(t, err)
	projectRole, err := store.ProjectRoleForPrincipal(ctx, project.ID, member.ID)
	require.NoError(t, err)
	require.NotNil(t, projectRole)
	require.Equal(t, domain.MemberRoleOwner, *projectRole)
}

func TestRBACProjectInviteRequiresProjectOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	store, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()
	workspace, project := createRBACWorkspaceProject(t, store, "invite-ws", "invite-project")
	owner := createRBACPrincipal(t, store, "oidc|project-owner", "project-owner@example.com")
	member := createRBACPrincipal(t, store, "oidc|project-member", "project-member@example.com")
	workspaceMemberProjectOwner := createRBACPrincipal(t, store, "oidc|workspace-member-project-owner", "workspace-member-project-owner@example.com")
	workspaceMemberOnly := createRBACPrincipal(t, store, "oidc|workspace-member-only", "workspace-member-only@example.com")
	require.NoError(t, store.CreateProjectMembership(ctx, &domain.ProjectMembership{ProjectID: project.ID, PrincipalID: owner.ID, Role: domain.MemberRoleOwner, CreatedBy: "test"}))
	require.NoError(t, store.CreateProjectMembership(ctx, &domain.ProjectMembership{ProjectID: project.ID, PrincipalID: member.ID, Role: domain.MemberRoleMember, CreatedBy: "test"}))
	require.NoError(t, store.CreateWorkspaceMembership(ctx, &domain.WorkspaceMembership{WorkspaceID: workspace.ID, PrincipalID: workspaceMemberProjectOwner.ID, Role: domain.MemberRoleMember, CreatedBy: "test"}))
	require.NoError(t, store.CreateProjectMembership(ctx, &domain.ProjectMembership{ProjectID: project.ID, PrincipalID: workspaceMemberProjectOwner.ID, Role: domain.MemberRoleOwner, CreatedBy: "test"}))
	require.NoError(t, store.CreateWorkspaceMembership(ctx, &domain.WorkspaceMembership{WorkspaceID: workspace.ID, PrincipalID: workspaceMemberOnly.ID, Role: domain.MemberRoleMember, CreatedBy: "test"}))

	api := testRBACAPI(store)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/invites", api.HandleCreateProjectInvite)

	body := bytes.NewBufferString(`{"email":"invitee@example.com"}`)
	ownerReq := requestWithPrincipal(http.MethodPost, "/v1/workspaces/invite-ws/projects/invite-project/invites", body, Principal{Subject: owner.Subject, Email: "project-owner@example.com", AuthMethod: "oidc"})
	ownerRecorder := httptest.NewRecorder()
	mux.ServeHTTP(ownerRecorder, ownerReq)
	require.Equal(t, http.StatusCreated, ownerRecorder.Code)

	memberBody := bytes.NewBufferString(`{"email":"invitee2@example.com"}`)
	memberReq := requestWithPrincipal(http.MethodPost, "/v1/workspaces/invite-ws/projects/invite-project/invites", memberBody, Principal{Subject: member.Subject, Email: "project-member@example.com", AuthMethod: "oidc"})
	memberRecorder := httptest.NewRecorder()
	mux.ServeHTTP(memberRecorder, memberReq)
	require.Equal(t, http.StatusForbidden, memberRecorder.Code)

	workspaceMemberProjectOwnerBody := bytes.NewBufferString(`{"email":"invitee3@example.com"}`)
	workspaceMemberProjectOwnerReq := requestWithPrincipal(http.MethodPost, "/v1/workspaces/invite-ws/projects/invite-project/invites", workspaceMemberProjectOwnerBody, Principal{Subject: workspaceMemberProjectOwner.Subject, Email: "workspace-member-project-owner@example.com", AuthMethod: "oidc"})
	workspaceMemberProjectOwnerRecorder := httptest.NewRecorder()
	mux.ServeHTTP(workspaceMemberProjectOwnerRecorder, workspaceMemberProjectOwnerReq)
	require.Equal(t, http.StatusCreated, workspaceMemberProjectOwnerRecorder.Code)

	workspaceMemberOnlyBody := bytes.NewBufferString(`{"email":"invitee4@example.com"}`)
	workspaceMemberOnlyReq := requestWithPrincipal(http.MethodPost, "/v1/workspaces/invite-ws/projects/invite-project/invites", workspaceMemberOnlyBody, Principal{Subject: workspaceMemberOnly.Subject, Email: "workspace-member-only@example.com", AuthMethod: "oidc"})
	workspaceMemberOnlyRecorder := httptest.NewRecorder()
	mux.ServeHTTP(workspaceMemberOnlyRecorder, workspaceMemberOnlyReq)
	require.Equal(t, http.StatusForbidden, workspaceMemberOnlyRecorder.Code)
}
