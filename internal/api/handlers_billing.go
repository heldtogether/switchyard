package api

import (
	"net/http"
	"time"
)

// HandleWorkspaceMonthToDateBilling handles GET /v1/workspaces/{workspace_slug}/billing/month-to-date
func (a *API) HandleWorkspaceMonthToDateBilling(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")

	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
		})
		return
	}
	if _, ok := a.requireWorkspaceAccess(w, r, workspace, false); !ok {
		return
	}

	monthKey := time.Now().UTC().Format("2006-01")
	mtd, err := a.store.GetWorkspaceMonthToDateBilling(r.Context(), workspace.ID, monthKey)
	if err != nil {
		a.logger.Error("failed to query workspace month-to-date billing", "workspace_id", workspace.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to query billing",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	writeJSON(w, http.StatusOK, WorkspaceMonthToDateBillingResponse{
		WorkspaceID:              mtd.WorkspaceID,
		MonthKey:                 mtd.MonthKey,
		CPUSeconds:               mtd.CPUSeconds,
		MemoryGBSeconds:          mtd.MemoryGBSeconds,
		GPUSeconds:               mtd.GPUSeconds,
		EstimatedTotalMinor:      mtd.EstimatedTotalMinor,
		EstimatedTotalMinorExact: mtd.EstimatedTotalMinorExact,
		Currency:                 mtd.Currency,
	})
}

// HandleRunBillingBreakdown handles GET /v1/workspaces/{workspace_slug}/projects/{project_slug}/runs/{run_slug}/billing
func (a *API) HandleRunBillingBreakdown(w http.ResponseWriter, r *http.Request) {
	workspaceSlug := r.PathValue("workspace_slug")
	projectSlug := r.PathValue("project_slug")
	runSlug := r.PathValue("run_slug")

	workspace, err := a.store.GetWorkspaceBySlug(r.Context(), workspaceSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Workspace not found",
			Code:    http.StatusNotFound,
		})
		return
	}
	project, err := a.store.GetProjectBySlug(r.Context(), workspace.ID, projectSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Project not found",
			Code:    http.StatusNotFound,
		})
		return
	}
	if _, ok := a.requireProjectAccess(w, r, workspace, project); !ok {
		return
	}

	run, err := a.store.GetRunBySlug(r.Context(), project.ID, runSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Run not found",
			Code:    http.StatusNotFound,
		})
		return
	}

	breakdown, err := a.store.GetRunBillingBreakdown(r.Context(), workspace.ID, project.ID, run.ID)
	if err != nil {
		a.logger.Error("failed to query run billing breakdown", "run_id", run.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to query run billing",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	items := make([]RunBillingLineItemResponse, 0, len(breakdown.Items))
	for _, item := range breakdown.Items {
		items = append(items, RunBillingLineItemResponse{
			JobID:                     item.JobID,
			CPUSeconds:                item.CPUSeconds,
			MemoryGBSeconds:           item.MemoryGBSeconds,
			GPUSeconds:                item.GPUSeconds,
			EstimatedCPUMinor:         item.EstimatedCPUMinor,
			EstimatedMemoryMinor:      item.EstimatedMemoryMinor,
			EstimatedGPUMinor:         item.EstimatedGPUMinor,
			EstimatedTotalMinor:       item.EstimatedTotalMinor,
			EstimatedCPUMinorExact:    item.EstimatedCPUMinorExact,
			EstimatedMemoryMinorExact: item.EstimatedMemoryMinorExact,
			EstimatedGPUMinorExact:    item.EstimatedGPUMinorExact,
			EstimatedTotalMinorExact:  item.EstimatedTotalMinorExact,
			PricingVersion:            item.PricingVersion,
			Currency:                  item.Currency,
			CreatedAt:                 item.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, RunBillingBreakdownResponse{
		WorkspaceID:              breakdown.WorkspaceID,
		ProjectID:                breakdown.ProjectID,
		RunID:                    breakdown.RunID,
		CPUSeconds:               breakdown.CPUSeconds,
		MemoryGBSeconds:          breakdown.MemoryGBSeconds,
		GPUSeconds:               breakdown.GPUSeconds,
		EstimatedTotalMinor:      breakdown.EstimatedTotalMinor,
		EstimatedTotalMinorExact: breakdown.EstimatedTotalMinorExact,
		Currency:                 breakdown.Currency,
		Items:                    items,
	})
}
