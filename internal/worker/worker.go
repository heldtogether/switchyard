package worker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	billingclient "github.com/heldtogether/switchyard/internal/billing"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/control"
	"github.com/heldtogether/switchyard/internal/executor"
	"github.com/heldtogether/switchyard/internal/registrysecrets"
	"github.com/heldtogether/switchyard/internal/storage/objectstore"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/storage/queue"
)

// Worker polls for jobs and executes them
type Worker struct {
	cfg           *config.Config
	queue         queue.Consumer
	store         *postgres.Store
	executor      executor.Executor
	storage       *objectstore.S3Store
	logger        *slog.Logger
	api           *APIClient
	nodeID        string
	hostname      string
	gpuTotal      int
	gpuDeviceIDs  []string
	cleanup       config.CleanupConfig
	secretCodec   *registrysecrets.Codec
	stripeEmitter billingclient.StripeMeterEmitter
	cancelSub     control.Subscriber
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc

	attempts   map[string]int
	attemptsMu sync.Mutex

	runningMu      sync.Mutex
	runningJobs    map[uuid.UUID]executor.RunRef
	pendingCancels map[uuid.UUID]control.CancelSignal
	cancellationBy string
}

// New creates a new Worker
func New(cfg *config.Config, q queue.Consumer, store *postgres.Store, exec executor.Executor, storage *objectstore.S3Store, logger *slog.Logger, api *APIClient, nodeID, hostname string, gpuTotal int, gpuDeviceIDs []string, secretCodec *registrysecrets.Codec, cancelSub control.Subscriber) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	cleanup := cfg.Executor.Docker.Cleanup

	var emitter billingclient.StripeMeterEmitter
	if cfg.Billing.Enabled && cfg.Billing.InvoicesEnabled && cfg.Billing.Stripe.APIKey != "" {
		emitter = billingclient.NewStripeSDKMeterEmitter(cfg.Billing.Stripe.APIKey)
	}

	return &Worker{
		cfg:            cfg,
		queue:          q,
		store:          store,
		executor:       exec,
		storage:        storage,
		logger:         logger,
		api:            api,
		nodeID:         nodeID,
		hostname:       hostname,
		gpuTotal:       gpuTotal,
		gpuDeviceIDs:   gpuDeviceIDs,
		cleanup:        cleanup,
		secretCodec:    secretCodec,
		stripeEmitter:  emitter,
		cancelSub:      cancelSub,
		ctx:            ctx,
		cancel:         cancel,
		attempts:       make(map[string]int),
		runningJobs:    make(map[uuid.UUID]executor.RunRef),
		pendingCancels: make(map[uuid.UUID]control.CancelSignal),
		cancellationBy: "worker",
	}
}

// Start begins polling for jobs
func (w *Worker) Start() error {
	w.logger.Info("worker starting", "concurrency", w.cfg.Worker.Concurrency)

	if w.api != nil {
		if err := w.api.RegisterWorker(w.ctx, RegisterWorkerRequest{
			NodeID:       w.nodeID,
			Hostname:     w.hostname,
			Executor:     w.cfg.Executor.Type,
			GPUTotal:     w.gpuTotal,
			GPUDeviceIDs: w.gpuDeviceIDs,
		}); err != nil {
			w.logger.Error("failed to register worker", "error", err)
		}

		w.wg.Add(1)
		go w.heartbeatLoop()
	}

	// Start Redis delay requeue loop (no-op for RabbitMQ)
	w.wg.Add(1)
	go w.requeueLoop()

	if w.cfg.Billing.Enabled && w.cfg.Billing.InvoicesEnabled && w.stripeEmitter != nil {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			runStripeMeterRetryLoop(w.ctx, w.store, w.stripeEmitter, w.cfg.Billing)
		}()
	}

	if w.cancelSub != nil {
		w.wg.Add(1)
		go w.cancelLoop()
	}

	// Recover any orphaned running jobs on startup
	if err := w.recoverOrphanedJobs(); err != nil {
		w.logger.Error("failed to recover orphaned jobs", "error", err)
	}

	// Start worker goroutines
	for i := 0; i < w.cfg.Worker.Concurrency; i++ {
		w.wg.Add(1)
		go w.workerLoop(i)
	}

	w.logger.Info("worker started")
	return nil
}

// Stop gracefully stops the worker
func (w *Worker) Stop() error {
	w.logger.Info("worker stopping")
	w.cancel()

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("worker stopped gracefully")
	case <-time.After(w.cfg.Worker.GracefulShutdown):
		w.logger.Warn("worker stop timeout, some jobs may be interrupted")
	}

	return nil
}

// workerLoop is the main loop for a worker goroutine
func (w *Worker) workerLoop(workerID int) {
	defer w.wg.Done()

	logger := w.logger.With("worker_id", workerID)
	logger.Info("worker loop started")

	for {
		select {
		case <-w.ctx.Done():
			logger.Info("worker loop stopped")
			return
		default:
			msg, err := w.queue.Pop(w.ctx, w.cfg.Worker.PollInterval)
			if err != nil {
				logger.Error("failed to pop from queue", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if msg == nil {
				continue
			}

			jobIDStr := msg.JobID()
			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				logger.Error("invalid job id", "job_id", jobIDStr, "error", err)
				_ = msg.Nack(false)
				continue
			}

			if w.api != nil {
				claimResp, claimErr := w.api.ClaimAllocation(w.ctx, AllocationClaimRequest{JobID: jobID, NodeID: w.nodeID})
				err = claimErr
				if err != nil {
					if err == ErrInsufficientGPU {
						retryAt := time.Now().Add(w.nextRetryDelay(jobIDStr))
						if delayErr := w.queue.Delay(w.ctx, jobIDStr, retryAt); delayErr != nil {
							logger.Error("failed to delay job", "job_id", jobIDStr, "error", delayErr)
						} else {
							logger.Info("delayed job due to insufficient GPU", "job_id", jobIDStr, "retry_at", retryAt)
						}
						_ = msg.Nack(false)
						continue
					}
					logger.Error("failed to claim allocation", "job_id", jobIDStr, "error", err)
					_ = msg.Nack(false)
					continue
				}
				if claimResp == nil {
					logger.Error("missing allocation claim response", "job_id", jobIDStr)
					_ = msg.Nack(false)
					continue
				}

				if err := msg.Ack(); err != nil {
					logger.Error("failed to ack message", "job_id", jobIDStr, "error", err)
					continue
				}

				w.resetRetry(jobIDStr)

				// Process the job
				logger.Info("processing job", "job_id", jobIDStr)
				if err := w.processJob(w.ctx, jobIDStr, claimResp.DeviceIDs); err != nil {
					logger.Error("failed to process job", "job_id", jobIDStr, "error", err)
				}

				if w.api != nil {
					if err := w.api.ReleaseAllocation(w.ctx, AllocationReleaseRequest{JobID: jobID, NodeID: w.nodeID}); err != nil {
						logger.Error("failed to release allocation", "job_id", jobIDStr, "error", err)
					}
				}
				continue
			}

			if err := msg.Ack(); err != nil {
				logger.Error("failed to ack message", "job_id", jobIDStr, "error", err)
				continue
			}

			w.resetRetry(jobIDStr)

			// Process the job
			logger.Info("processing job", "job_id", jobIDStr)
			if err := w.processJob(w.ctx, jobIDStr, nil); err != nil {
				logger.Error("failed to process job", "job_id", jobIDStr, "error", err)
			}

			if w.api != nil {
				if err := w.api.ReleaseAllocation(w.ctx, AllocationReleaseRequest{JobID: jobID, NodeID: w.nodeID}); err != nil {
					logger.Error("failed to release allocation", "job_id", jobIDStr, "error", err)
				}
			}
		}
	}
}

// processJob handles a single job execution
func (w *Worker) processJob(ctx context.Context, jobIDStr string, gpuDeviceIDs []string) error {
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		return fmt.Errorf("invalid job ID: %w", err)
	}

	// Wrap S3Store to match ObjectStorage interface
	storageAdapter := &s3StorageAdapter{store: w.storage}
	processor := NewProcessor(w.store, w.executor, storageAdapter, w.logger, w.cfg.API.BaseURL, w.cfg.Storage.Bucket, w.nodeID, w.cleanup)
	processor.SetSecretCodec(w.secretCodec)
	processor.ConfigureBilling(w.cfg.Billing, w.store)
	processor.SetRunHooks(
		func(jobID uuid.UUID, ref executor.RunRef) {
			w.registerRunningJob(jobID, ref)
		},
		func(jobID uuid.UUID) {
			w.unregisterRunningJob(jobID)
		},
	)
	return processor.ProcessWithAllocation(ctx, jobID, gpuDeviceIDs)
}

func (w *Worker) cancelLoop() {
	defer w.wg.Done()
	logger := w.logger.With("component", "cancel_loop")
	err := w.cancelSub.ConsumeJobCancels(w.ctx, w.nodeID, func(ctx context.Context, signal control.CancelSignal) error {
		if err := w.store.CreateJobCancellationEvent(ctx, postgres.JobCancellationEvent{
			JobID:       signal.JobID,
			EventType:   "acknowledged",
			RequestedBy: signal.RequestedBy,
			NodeID:      &w.nodeID,
			Message:     stringPtr("worker received cancellation signal"),
		}); err != nil {
			logger.Warn("failed to record cancellation ack event", "job_id", signal.JobID, "error", err)
		}
		return w.cancelRunningJob(ctx, signal)
	})
	if err != nil && err != context.Canceled {
		logger.Error("cancel loop stopped with error", "error", err)
	}
}

func (w *Worker) registerRunningJob(jobID uuid.UUID, ref executor.RunRef) {
	w.runningMu.Lock()
	w.runningJobs[jobID] = ref
	pending, hasPending := w.pendingCancels[jobID]
	if hasPending {
		delete(w.pendingCancels, jobID)
	}
	w.runningMu.Unlock()

	if hasPending {
		_ = w.cancelRunningJob(w.ctx, pending)
	}
}

func (w *Worker) unregisterRunningJob(jobID uuid.UUID) {
	w.runningMu.Lock()
	defer w.runningMu.Unlock()
	delete(w.runningJobs, jobID)
	delete(w.pendingCancels, jobID)
}

func (w *Worker) cancelRunningJob(ctx context.Context, signal control.CancelSignal) error {
	w.runningMu.Lock()
	ref, ok := w.runningJobs[signal.JobID]
	if !ok {
		w.pendingCancels[signal.JobID] = signal
		w.runningMu.Unlock()
		return nil
	}
	w.runningMu.Unlock()

	if err := w.executor.Cancel(ctx, ref); err != nil {
		_ = w.store.CreateJobCancellationEvent(ctx, postgres.JobCancellationEvent{
			JobID:       signal.JobID,
			EventType:   "failed",
			RequestedBy: signal.RequestedBy,
			NodeID:      &w.nodeID,
			ExecutorRef: &ref.Reference,
			Message:     stringPtr(fmt.Sprintf("executor cancel failed: %v", err)),
		})
		return err
	}

	_ = w.store.CreateJobCancellationEvent(ctx, postgres.JobCancellationEvent{
		JobID:       signal.JobID,
		EventType:   "completed",
		RequestedBy: signal.RequestedBy,
		NodeID:      &w.nodeID,
		ExecutorRef: &ref.Reference,
		Message:     stringPtr("executor cancel issued by worker"),
	})
	return nil
}

func (w *Worker) heartbeatLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.cfg.Worker.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if err := w.api.Heartbeat(w.ctx, WorkerHeartbeatRequest{NodeID: w.nodeID, GPUTotal: w.gpuTotal, GPUDeviceIDs: w.gpuDeviceIDs}); err != nil {
				w.logger.Error("failed to send heartbeat", "error", err)
			}
		}
	}
}

func (w *Worker) requeueLoop() {
	defer w.wg.Done()

	interval := w.cfg.Queue.RequeueInterval
	batch := w.cfg.Queue.RequeueBatch
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if batch <= 0 {
		batch = 100
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			count, err := w.queue.RequeueReady(w.ctx, batch)
			if err != nil {
				w.logger.Error("requeue loop error", "error", err)
				continue
			}
			if count > 0 {
				w.logger.Info("requeued delayed jobs", "count", count)
			}
		}
	}
}

func (w *Worker) nextRetryDelay(jobID string) time.Duration {
	w.attemptsMu.Lock()
	defer w.attemptsMu.Unlock()

	attempt := w.attempts[jobID]
	w.attempts[jobID] = attempt + 1

	return retryDelay(attempt, w.cfg.Worker.RetryBaseDelay, w.cfg.Worker.RetryMaxDelay, w.cfg.Worker.RetryJitter)
}

func (w *Worker) resetRetry(jobID string) {
	w.attemptsMu.Lock()
	defer w.attemptsMu.Unlock()
	delete(w.attempts, jobID)
}

func stringPtr(v string) *string {
	return &v
}

// s3StorageAdapter adapts *objectstore.S3Store to ObjectStorage interface
type s3StorageAdapter struct {
	store *objectstore.S3Store
}

func (a *s3StorageAdapter) Upload(ctx context.Context, key string, r io.Reader, contentType string) error {
	return a.store.Upload(ctx, key, r, contentType)
}

func (a *s3StorageAdapter) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return a.store.Download(ctx, key)
}

func (a *s3StorageAdapter) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return a.store.PresignedURL(ctx, key, expiry)
}

func (a *s3StorageAdapter) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	objects, err := a.store.List(ctx, prefix)
	if err != nil {
		return nil, err
	}

	result := make([]ObjectInfo, len(objects))
	for i, obj := range objects {
		result[i] = ObjectInfo{
			Key:          obj.Key,
			SizeBytes:    obj.SizeBytes,
			ContentType:  obj.ContentType,
			LastModified: obj.LastModified,
		}
	}
	return result, nil
}

// recoverOrphanedJobs checks for jobs stuck in RUNNING state on startup
func (w *Worker) recoverOrphanedJobs() error {
	w.logger.Info("recovering orphaned jobs")

	jobs, err := w.store.GetRunningJobs(w.ctx)
	if err != nil {
		return fmt.Errorf("failed to get running jobs: %w", err)
	}

	if len(jobs) == 0 {
		w.logger.Info("no orphaned jobs found")
		return nil
	}

	w.logger.Info("found orphaned jobs", "count", len(jobs))

	updatedRuns := make(map[uuid.UUID]struct{})

	for _, job := range jobs {
		logger := w.logger.With("job_id", job.ID)

		if job.ExecutorRef == nil {
			// Job never started execution
			logger.Info("marking job as failed (no executor reference)")
			msg := "worker crashed before job started"
			if err := w.store.UpdateJobStatus(w.ctx, job.ID, "FAILED", &msg); err != nil {
				logger.Error("failed to update orphaned job", "error", err)
			} else {
				updatedRuns[job.RunID] = struct{}{}
			}
			continue
		}

		// Check executor status
		ref := executor.RunRef{
			ExecutorType: string(job.Executor),
			Reference:    *job.ExecutorRef,
		}

		status, err := w.executor.Status(w.ctx, ref)
		if err != nil {
			logger.Error("failed to check executor status", "error", err)
			continue
		}

		// Update job based on executor status
		switch status {
		case executor.StatusSuccess:
			logger.Info("job completed successfully (recovery)")
			job.Status = "SUCCEEDED"
			finishedAt := time.Now()
			job.FinishedAt = &finishedAt
		case executor.StatusFailed, executor.StatusCancelled, executor.StatusTimeout:
			logger.Info("job failed (recovery)")
			job.Status = "FAILED"
			finishedAt := time.Now()
			job.FinishedAt = &finishedAt
			msg := "job failed during worker downtime"
			job.StatusMessage = &msg
		case executor.StatusUnknown:
			logger.Info("job executor not found (recovery)")
			job.Status = "FAILED"
			finishedAt := time.Now()
			job.FinishedAt = &finishedAt
			msg := "executor service not found after worker restart"
			job.StatusMessage = &msg
		default:
			logger.Info("job still running (recovery)", "status", status)
			continue
		}

		if err := w.store.UpdateJob(w.ctx, job); err != nil {
			logger.Error("failed to update recovered job", "error", err)
		} else {
			updatedRuns[job.RunID] = struct{}{}
		}

		if w.api != nil {
			if err := w.api.ReleaseAllocation(w.ctx, AllocationReleaseRequest{JobID: job.ID, NodeID: w.nodeID}); err != nil {
				logger.Error("failed to release allocation (recovery)", "error", err)
			}
		}
	}

	for runID := range updatedRuns {
		if err := w.store.RecomputeRunStatus(w.ctx, runID); err != nil {
			w.logger.Error("failed to recompute run status", "run_id", runID, "error", err)
		}
	}

	return nil
}
