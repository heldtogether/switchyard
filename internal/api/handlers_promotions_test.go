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
	"github.com/heldtogether/switchyard/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestHandleCreatePromotionAndListCurrent(t *testing.T) {
	pg := testutil.SetupTestPostgres(t)
	defer pg.Cleanup(t)

	store, err := postgres.New(pg.ConnString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	workspace, err := store.GetWorkspaceBySlug(ctx, "default")
	require.NoError(t, err)

	project := &domain.Project{
		ID:          uuid.New(),
		WorkspaceID: workspace.ID,
		Slug:        "promo-project",
		Name:        "Promo Project",
		CreatedBy:   "test-user",
	}
	require.NoError(t, store.CreateProject(ctx, project))

	run := &domain.Run{
		ID:        uuid.New(),
		ProjectID: project.ID,
		Slug:      "promo-run",
		Name:      "Promo Run",
		Status:    domain.RunStatusSucceeded,
		CreatedBy: "test-user",
	}
	require.NoError(t, store.CreateRun(ctx, run))

	job := &domain.Job{
		ID:          uuid.New(),
		RunID:       run.ID,
		CreatedBy:   "test-user",
		Status:      domain.JobStatusSucceeded,
		Image:       "alpine:latest",
		Outputs:     []string{"/outputs"},
		TimeoutSecs: 30,
		Executor:    domain.ExecutorTypeDocker,
	}
	require.NoError(t, store.CreateJob(ctx, job))
	require.NoError(t, store.SaveArtefacts(ctx, job.ID, []domain.Artefact{{
		Path:        "outputs/model.bin",
		ObjectKey:   "runs/promo-run/outputs/model.bin",
		SizeBytes:   1024,
		ContentType: "application/octet-stream",
	}}))

	api := &API{
		cfg:    &config.Config{},
		store:  store,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions", api.HandleCreatePromotion)
	mux.HandleFunc("GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions", api.HandleListCurrentPromotions)

	body := map[string]any{
		"channel": "staging",
		"run_id":  run.ID,
		"note":    "release candidate",
		"artefacts": []map[string]any{
			{
				"logical_key": "model",
				"job_id":      job.ID,
				"path":        "outputs/model.bin",
			},
		},
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)

	createReq := httptest.NewRequest(http.MethodPost, "/v1/workspaces/default/projects/promo-project/promotions", bytes.NewReader(payload))
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created PromotionEventResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	require.Equal(t, "staging", created.Channel)
	require.Equal(t, run.ID, created.RunID)
	require.Equal(t, "release candidate", *created.Note)
	require.Len(t, created.Artefacts, 1)
	require.Equal(t, "model", created.Artefacts[0].LogicalKey)

	listReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces/default/projects/promo-project/promotions", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)

	var listed ListCurrentPromotionsResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listed))
	require.Len(t, listed.Promotions, 1)
	require.Equal(t, "staging", listed.Promotions[0].Channel)
	require.Equal(t, created.ID, listed.Promotions[0].Event.ID)
	require.Len(t, listed.Promotions[0].Event.Artefacts, 1)
	require.Equal(t, "model", listed.Promotions[0].Event.Artefacts[0].LogicalKey)
}

func TestHandleCreatePromotionRejectsDuplicateLogicalKey(t *testing.T) {
	pg := testutil.SetupTestPostgres(t)
	defer pg.Cleanup(t)

	store, err := postgres.New(pg.ConnString)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	workspace, err := store.GetWorkspaceBySlug(ctx, "default")
	require.NoError(t, err)

	project := &domain.Project{ID: uuid.New(), WorkspaceID: workspace.ID, Slug: "promo-project", Name: "Promo", CreatedBy: "test-user"}
	require.NoError(t, store.CreateProject(ctx, project))
	run := &domain.Run{ID: uuid.New(), ProjectID: project.ID, Slug: "run-1", Name: "Run", Status: domain.RunStatusSucceeded, CreatedBy: "test-user"}
	require.NoError(t, store.CreateRun(ctx, run))

	api := &API{cfg: &config.Config{}, store: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces/{workspace_slug}/projects/{project_slug}/promotions", api.HandleCreatePromotion)

	body := `{"channel":"dev","run_id":"` + run.ID.String() + `","artefacts":[{"logical_key":"model","job_id":"` + uuid.NewString() + `","path":"a"},{"logical_key":"model","job_id":"` + uuid.NewString() + `","path":"b"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/default/projects/promo-project/promotions", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
