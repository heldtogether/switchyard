package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/lib/pq"
)

func (a *API) HandleCreatePromotion(w http.ResponseWriter, r *http.Request) {
	workspace, project, ok := a.authorizePromotionProjectScope(w, r)
	if !ok {
		return
	}

	var req CreatePromotionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Invalid JSON body", Code: http.StatusBadRequest})
		return
	}

	channel := domain.PromotionChannel(strings.ToLower(strings.TrimSpace(req.Channel)))
	if !channel.Valid() {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "invalid channel", Code: http.StatusBadRequest})
		return
	}

	run, err := a.resolvePromotionRun(r, project.ID, req.RunID, req.RunSlug)
	if err != nil {
		if errors.Is(err, postgres.ErrPromotionNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Run not found", Code: http.StatusNotFound})
			return
		}
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: err.Error(), Code: http.StatusBadRequest})
		return
	}

	seen := make(map[string]struct{}, len(req.Artefacts))
	inputs := make([]postgres.PromotionEventCreateArtefactInput, 0, len(req.Artefacts))
	for _, art := range req.Artefacts {
		logical := strings.ToLower(strings.TrimSpace(art.LogicalKey))
		if logical == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "logical_key is required for promoted artefacts", Code: http.StatusBadRequest})
			return
		}
		if art.JobID == uuid.Nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "job_id is required for promoted artefacts", Code: http.StatusBadRequest})
			return
		}
		path := strings.TrimSpace(art.Path)
		if path == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "path is required for promoted artefacts", Code: http.StatusBadRequest})
			return
		}
		if _, exists := seen[logical]; exists {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "duplicate logical_key in request", Code: http.StatusBadRequest})
			return
		}
		seen[logical] = struct{}{}
		inputs = append(inputs, postgres.PromotionEventCreateArtefactInput{LogicalKey: logical, JobID: art.JobID, Path: path})
	}

	var promotedByPrincipalID *uuid.UUID
	if pid, found, err := a.currentPrincipalID(r); err == nil && found {
		promotedByPrincipalID = &pid
	}

	note := normalizeOptionalText(req.Note)
	event, err := a.store.CreatePromotionEventAndSetCurrent(r.Context(), postgres.PromotionEventCreateInput{
		WorkspaceID:           workspace.ID,
		ProjectID:             project.ID,
		Channel:               channel,
		RunID:                 run.ID,
		Note:                  note,
		PromotedBy:            ActorFromRequest(r),
		PromotedByPrincipalID: promotedByPrincipalID,
		Artefacts:             inputs,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, ErrorResponse{Error: "conflict", Message: "duplicate promotion key", Code: http.StatusConflict})
			return
		}
		a.logger.Error("failed to create promotion", "error", err, "workspace", workspace.Slug, "project", project.Slug)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to create promotion", Code: http.StatusInternalServerError})
		return
	}

	writeJSON(w, http.StatusCreated, toPromotionEventResponse(*event))
}

func (a *API) HandleListCurrentPromotions(w http.ResponseWriter, r *http.Request) {
	workspace, project, ok := a.authorizePromotionProjectScope(w, r)
	if !ok {
		return
	}

	promotions, err := a.store.ListCurrentPromotions(r.Context(), workspace.ID, project.ID)
	if err != nil {
		a.logger.Error("failed to list current promotions", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to list promotions", Code: http.StatusInternalServerError})
		return
	}

	resp := make([]CurrentPromotionResponse, 0, len(promotions))
	for _, item := range promotions {
		resp = append(resp, CurrentPromotionResponse{
			ProjectID: item.ProjectID,
			Channel:   string(item.Channel),
			Event:     toPromotionEventResponse(item.Event),
		})
	}

	writeJSON(w, http.StatusOK, ListCurrentPromotionsResponse{ProjectID: project.ID, Promotions: resp})
}

func (a *API) HandleGetCurrentPromotionByChannel(w http.ResponseWriter, r *http.Request) {
	workspace, project, ok := a.authorizePromotionProjectScope(w, r)
	if !ok {
		return
	}

	channel := domain.PromotionChannel(strings.ToLower(strings.TrimSpace(r.PathValue("channel"))))
	if !channel.Valid() {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "invalid channel", Code: http.StatusBadRequest})
		return
	}

	promotion, err := a.store.GetCurrentPromotionByChannel(r.Context(), workspace.ID, project.ID, channel)
	if err != nil {
		if errors.Is(err, postgres.ErrPromotionNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Promotion not found", Code: http.StatusNotFound})
			return
		}
		a.logger.Error("failed to get channel promotion", "error", err, "channel", channel)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to fetch promotion", Code: http.StatusInternalServerError})
		return
	}

	writeJSON(w, http.StatusOK, CurrentPromotionResponse{
		ProjectID: promotion.ProjectID,
		Channel:   string(promotion.Channel),
		Event:     toPromotionEventResponse(promotion.Event),
	})
}

func (a *API) HandleListPromotionHistory(w http.ResponseWriter, r *http.Request) {
	workspace, project, ok := a.authorizePromotionProjectScope(w, r)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var channel *domain.PromotionChannel
	if channelRaw := strings.TrimSpace(r.URL.Query().Get("channel")); channelRaw != "" {
		c := domain.PromotionChannel(strings.ToLower(channelRaw))
		if !c.Valid() {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "invalid channel", Code: http.StatusBadRequest})
			return
		}
		channel = &c
	}

	events, err := a.store.ListPromotionHistory(r.Context(), workspace.ID, project.ID, channel, limit, offset)
	if err != nil {
		a.logger.Error("failed to list promotion history", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to list promotion history", Code: http.StatusInternalServerError})
		return
	}
	total, err := a.store.PromotionHistoryCount(r.Context(), workspace.ID, project.ID, channel)
	if err != nil {
		a.logger.Error("failed to count promotion history", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to count promotion history", Code: http.StatusInternalServerError})
		return
	}

	resp := make([]PromotionEventResponse, 0, len(events))
	for _, event := range events {
		resp = append(resp, toPromotionEventResponse(event))
	}

	writeJSON(w, http.StatusOK, PromotionHistoryResponse{
		Events: resp,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (a *API) HandleResolvePromotedArtefact(w http.ResponseWriter, r *http.Request) {
	workspace, project, ok := a.authorizePromotionProjectScope(w, r)
	if !ok {
		return
	}

	channel := domain.PromotionChannel(strings.ToLower(strings.TrimSpace(r.PathValue("channel"))))
	if !channel.Valid() {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "invalid channel", Code: http.StatusBadRequest})
		return
	}
	logicalKey := strings.ToLower(strings.TrimSpace(r.PathValue("logical_key")))
	if logicalKey == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "validation_error", Message: "logical_key is required", Code: http.StatusBadRequest})
		return
	}

	resolved, err := a.store.ResolvePromotedArtefact(r.Context(), workspace.ID, project.ID, channel, logicalKey)
	if err != nil {
		if errors.Is(err, postgres.ErrPromotionNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Promoted artefact not found", Code: http.StatusNotFound})
			return
		}
		a.logger.Error("failed to resolve promoted artefact", "error", err, "channel", channel, "logical_key", logicalKey)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to resolve promoted artefact", Code: http.StatusInternalServerError})
		return
	}

	ttl := a.cfg.API.Promotions.PresignedURLTTL
	downloadURL, err := a.storage.PresignedURL(r.Context(), resolved.ObjectKey, ttl)
	if err != nil {
		a.logger.Error("failed to generate promotion artefact presigned URL", "error", err, "key", resolved.ObjectKey)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: "Failed to resolve promoted artefact", Code: http.StatusInternalServerError})
		return
	}

	expiresAt := time.Now().UTC().Add(ttl)
	writeJSON(w, http.StatusOK, ResolvedPromotedArtefactResponse{
		Channel:              string(resolved.Channel),
		LogicalKey:           resolved.LogicalKey,
		PromotionEventID:     resolved.PromotionEventID,
		RunID:                resolved.RunID,
		JobID:                resolved.JobID,
		Path:                 resolved.Path,
		ObjectKey:            resolved.ObjectKey,
		SizeBytes:            resolved.SizeBytes,
		ContentType:          resolved.ContentType,
		PromotedAt:           resolved.PromotedAt,
		PromotedBy:           resolved.PromotedBy,
		DownloadURL:          downloadURL,
		DownloadURLExpiresAt: expiresAt,
	})
}

func (a *API) authorizePromotionProjectScope(w http.ResponseWriter, r *http.Request) (*domain.Workspace, *domain.Project, bool) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")

	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Workspace not found", Code: http.StatusNotFound})
		return nil, nil, false
	}
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, false); !ok {
		return nil, nil, false
	}

	project, err := a.store.GetProjectBySlug(r.Context(), workspace.ID, projectSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "not_found", Message: "Project not found", Code: http.StatusNotFound})
		return nil, nil, false
	}
	if _, ok := a.requireProjectAccess(w, r, workspace, project); !ok {
		return nil, nil, false
	}

	return workspace, project, true
}

func (a *API) resolvePromotionRun(r *http.Request, projectID uuid.UUID, runID *uuid.UUID, runSlug string) (*domain.Run, error) {
	if runID == nil && strings.TrimSpace(runSlug) == "" {
		return nil, errors.New("run_id or run_slug is required")
	}

	var run *domain.Run
	if runID != nil {
		found, err := a.store.GetRun(r.Context(), *runID)
		if err != nil {
			return nil, postgres.ErrPromotionNotFound
		}
		run = found
	}
	if strings.TrimSpace(runSlug) != "" {
		found, err := a.store.GetRunBySlug(r.Context(), projectID, strings.TrimSpace(runSlug))
		if err != nil {
			return nil, postgres.ErrPromotionNotFound
		}
		if run != nil && run.ID != found.ID {
			return nil, errors.New("run_id and run_slug refer to different runs")
		}
		run = found
	}
	if run == nil {
		return nil, postgres.ErrPromotionNotFound
	}
	if run.ProjectID != projectID {
		return nil, errors.New("run does not belong to project")
	}
	return run, nil
}

func normalizeOptionalText(input *string) *string {
	if input == nil {
		return nil
	}
	v := strings.TrimSpace(*input)
	if v == "" {
		return nil
	}
	return &v
}

func toPromotionEventResponse(event domain.PromotionEvent) PromotionEventResponse {
	artefacts := make([]PromotionArtefactResponse, 0, len(event.Artefacts))
	for _, art := range event.Artefacts {
		artefacts = append(artefacts, PromotionArtefactResponse{
			LogicalKey:  art.LogicalKey,
			JobID:       art.JobID,
			Path:        art.Path,
			ObjectKey:   art.ObjectKey,
			SizeBytes:   art.SizeBytes,
			ContentType: art.ContentType,
		})
	}
	return PromotionEventResponse{
		ID:                    event.ID,
		WorkspaceID:           event.WorkspaceID,
		ProjectID:             event.ProjectID,
		Channel:               string(event.Channel),
		RunID:                 event.RunID,
		Note:                  event.Note,
		PromotedBy:            event.PromotedBy,
		PromotedByPrincipalID: event.PromotedByPrincipalID,
		CreatedAt:             event.CreatedAt,
		Artefacts:             artefacts,
	}
}
