package worker

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
	"github.com/heldtogether/switchyard/internal/executor"
	"github.com/heldtogether/switchyard/internal/version"
)

// Processor handles individual job execution
type Processor struct {
	store      JobStore
	executor   executor.Executor
	storage    ObjectStorage
	logger     *slog.Logger
	apiBaseURL string
	bucket     string
	nodeID     string
	cleanup    config.CleanupConfig
}

// NewProcessor creates a new job processor
func NewProcessor(store JobStore, exec executor.Executor, storage ObjectStorage, logger *slog.Logger, apiBaseURL string, bucket string, nodeID string, cleanup config.CleanupConfig) *Processor {
	return &Processor{
		store:      store,
		executor:   exec,
		storage:    storage,
		logger:     logger,
		apiBaseURL: apiBaseURL,
		bucket:     bucket,
		nodeID:     nodeID,
		cleanup:    cleanup,
	}
}

// Process executes a single job from start to finish
func (p *Processor) Process(ctx context.Context, jobID uuid.UUID) error {
	logger := p.logger.With("job_id", jobID)
	logger.Info("starting job processing")

	// 1. Fetch job from database
	job, err := p.store.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	// 2. Load run context
	run, err := p.store.GetRun(ctx, job.RunID)
	if err != nil {
		return fmt.Errorf("failed to fetch run: %w", err)
	}

	// 3. Load project context
	project, err := p.store.GetProject(ctx, run.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to fetch project: %w", err)
	}

	// 4. Load workspace context
	workspace, err := p.store.GetWorkspace(ctx, project.WorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to fetch workspace: %w", err)
	}

	logger = logger.With(
		"image", job.Image,
		"created_by", job.CreatedBy,
		"workspace", workspace.Slug,
		"project", project.Slug,
		"run", run.Slug,
	)

	// 5. Update status to RUNNING
	job.Status = domain.JobStatusRunning
	startedAt := time.Now()
	job.StartedAt = &startedAt

	if err := p.store.UpdateJob(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status to RUNNING: %w", err)
	}
	if err := p.store.RecomputeRunStatus(ctx, job.RunID); err != nil {
		logger.Error("failed to recompute run status", "error", err)
		return fmt.Errorf("failed to recompute run status: %w", err)
	}

	logger.Info("job status updated to RUNNING")

	// 6. Build executor run spec
	var registryAuth *domain.RegistryAuth
	if job.RegistrySecretID != nil {
		secret, err := p.store.GetRegistrySecret(ctx, *job.RegistrySecretID)
		if err != nil {
			return p.failJob(ctx, job, fmt.Errorf("failed to load registry secret: %w", err))
		}
		registryAuth = &domain.RegistryAuth{
			Host:     secret.Host,
			Username: secret.Username,
			Password: secret.PasswordEncrypted,
		}
	}

	spec := executor.RunSpec{
		JobID:             job.ID.String(),
		Image:             job.Image,
		ImageDigest:       stringPtrValue(job.ImageDigest),
		Command:           job.Command,
		Env:               job.Env,
		Outputs:           job.Outputs,
		CPU:               stringPtrValue(job.CPULimit),
		Memory:            stringPtrValue(job.MemoryLimit),
		GPUCount:          job.GPUCount,
		Timeout:           time.Duration(job.TimeoutSecs) * time.Second,
		RegistryAuth:      registryAuth,
		CreatedAt:         job.CreatedAt,
		ArtefactPrefix:    stringPtrValue(job.ArtefactPrefix),
		Bucket:            p.bucket,
		APIBaseURL:        p.apiBaseURL,
		SwitchyardVersion: version.Version,
		ExecutorType:      string(job.Executor),
		NodeID:            p.nodeID,
	}

	// 7. Create executor run
	ref, err := p.executor.CreateRun(ctx, spec)
	if err != nil {
		return p.failJob(ctx, job, fmt.Errorf("failed to create executor run: %w", err))
	}

	logger.Info("executor run created", "executor_ref", ref.Reference)

	// 8. Update job with executor reference
	job.ExecutorRef = &ref.Reference
	if err := p.store.UpdateJob(ctx, job); err != nil {
		logger.Error("failed to update executor reference", "error", err)
		// Non-fatal, continue
	}

	// 9. Wait for job completion with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()

	logger.Info("waiting for job completion", "timeout", spec.Timeout)
	result, err := p.executor.Wait(timeoutCtx, ref)

	// 10. Collect logs (even on failure)
	logger.Info("collecting logs")
	logKey := fmt.Sprintf("workspaces/%s/projects/%s/runs/%s/jobs/%s/logs.txt",
		workspace.Slug, project.Slug, run.Slug, job.ID)
	logBuf := &bytes.Buffer{}

	if logErr := p.executor.GetLogs(ctx, ref, logBuf); logErr != nil {
		logger.Error("failed to collect logs", "error", logErr)
	} else {
		if uploadErr := p.storage.Upload(ctx, logKey, logBuf, "text/plain"); uploadErr != nil {
			logger.Error("failed to upload logs", "error", uploadErr)
		} else {
			job.LogObjectKey = &logKey
			logger.Info("logs uploaded", "key", logKey, "size_bytes", logBuf.Len())
		}
	}

	// 11. Handle execution result
	finishedAt := time.Now()
	job.FinishedAt = &finishedAt

	if err == context.DeadlineExceeded {
		// Timeout
		logger.Warn("job timed out")
		job.Status = domain.JobStatusTimeout
		msg := fmt.Sprintf("job exceeded timeout of %s", spec.Timeout)
		job.StatusMessage = &msg

		// Try to cancel the executor
		if cancelErr := p.executor.Cancel(ctx, ref); cancelErr != nil {
			logger.Error("failed to cancel timed out job", "error", cancelErr)
		}
	} else if err != nil {
		// Other error
		logger.Error("job execution failed", "error", err)
		return p.failJob(ctx, job, err)
	} else {
		// Check result
		if result.ExitCode != 0 {
			logger.Warn("job failed with non-zero exit code", "exit_code", result.ExitCode)
			job.Status = domain.JobStatusFailed
			job.ExitCode = &result.ExitCode
			msg := fmt.Sprintf("container exited with code %d", result.ExitCode)
			job.StatusMessage = &msg
		} else {
			// Success - collect outputs
			logger.Info("job succeeded, collecting outputs")
			job.Status = domain.JobStatusSucceeded
			job.ExitCode = &result.ExitCode

			// Collect output artefacts
			outputPrefix := fmt.Sprintf("workspaces/%s/projects/%s/runs/%s/jobs/%s/outputs",
				workspace.Slug, project.Slug, run.Slug, job.ID)
			outputSpec := executor.OutputSpec{
				Paths:       job.Outputs,
				ObjectStore: NewS3Adapter(p.storage),
				KeyPrefix:   outputPrefix,
			}

			artefacts, collectErr := p.executor.CollectOutputs(ctx, ref, outputSpec)
			if collectErr != nil {
				logger.Error("failed to collect outputs", "error", collectErr)
			} else {
				logger.Info("outputs collected", "count", len(artefacts))

				// Save artefacts to database
				if len(artefacts) > 0 {
					if saveErr := p.store.SaveArtefacts(ctx, job.ID, artefacts); saveErr != nil {
						logger.Error("failed to save artefacts", "error", saveErr)
					}

					job.ArtefactPrefix = &outputPrefix
				}
			}
		}
	}

	// 12. Update final job status
	if err := p.store.UpdateJob(ctx, job); err != nil {
		logger.Error("failed to update final job status", "error", err)
		return err
	}
	if err := p.store.RecomputeRunStatus(ctx, job.RunID); err != nil {
		logger.Error("failed to recompute run status", "error", err)
		return err
	}

	logger.Info("job processing complete", "status", job.Status, "duration", job.FinishedAt.Sub(*job.StartedAt))

	// 13. Cleanup executor resources
	if shouldCleanup(p.cleanup, job.Status) {
		if cleanupErr := p.executor.Cleanup(ctx, ref); cleanupErr != nil {
			logger.Error("failed to cleanup executor resources", "error", cleanupErr)
		}
	} else {
		logger.Info("skipping executor cleanup", "status", job.Status)
	}

	return nil
}

// failJob marks a job as failed and updates the database
func (p *Processor) failJob(ctx context.Context, job *domain.Job, err error) error {
	logger := p.logger.With("job_id", job.ID)

	job.Status = domain.JobStatusFailed
	finishedAt := time.Now()
	job.FinishedAt = &finishedAt
	msg := err.Error()
	job.StatusMessage = &msg

	if updateErr := p.store.UpdateJob(ctx, job); updateErr != nil {
		logger.Error("failed to update job status to FAILED", "error", updateErr)
		return fmt.Errorf("original error: %w, update error: %v", err, updateErr)
	}
	if recomputeErr := p.store.RecomputeRunStatus(ctx, job.RunID); recomputeErr != nil {
		logger.Error("failed to recompute run status", "error", recomputeErr)
		return fmt.Errorf("original error: %w, recompute error: %v", err, recomputeErr)
	}

	logger.Info("job marked as failed")
	return err
}

// Helper function to safely dereference string pointers
func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func shouldCleanup(cfg config.CleanupConfig, status domain.JobStatus) bool {
	switch status {
	case domain.JobStatusSucceeded:
		return cfg.RemoveOnComplete
	case domain.JobStatusFailed, domain.JobStatusCancelled, domain.JobStatusTimeout:
		return !cfg.KeepFailedServices
	default:
		return true
	}
}
