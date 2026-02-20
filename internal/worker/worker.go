package worker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/executor"
	"github.com/heldtogether/switchyard/internal/storage/objectstore"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/storage/queue"
)

// Worker polls for jobs and executes them
type Worker struct {
	cfg      *config.Config
	queue    *queue.RedisQueue
	store    *postgres.Store
	executor executor.Executor
	storage  *objectstore.S3Store
	logger   *slog.Logger
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a new Worker
func New(cfg *config.Config, q *queue.RedisQueue, store *postgres.Store, exec executor.Executor, storage *objectstore.S3Store, logger *slog.Logger) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		cfg:      cfg,
		queue:    q,
		store:    store,
		executor: exec,
		storage:  storage,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins polling for jobs
func (w *Worker) Start() error {
	w.logger.Info("worker starting", "concurrency", w.cfg.Worker.Concurrency)

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
			// Pop job from queue (blocking with timeout)
			jobID, err := w.queue.Pop(w.ctx, w.cfg.Worker.PollInterval)
			if err != nil {
				logger.Error("failed to pop from queue", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if jobID == "" {
				// Timeout, no jobs available
				continue
			}

			// Process the job
			logger.Info("processing job", "job_id", jobID)
			if err := w.processJob(w.ctx, jobID); err != nil {
				logger.Error("failed to process job", "job_id", jobID, "error", err)
			}
		}
	}
}

// processJob handles a single job execution
func (w *Worker) processJob(ctx context.Context, jobIDStr string) error {
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		return fmt.Errorf("invalid job ID: %w", err)
	}

	// Wrap S3Store to match ObjectStorage interface
	storageAdapter := &s3StorageAdapter{store: w.storage}
	processor := NewProcessor(w.store, w.executor, storageAdapter, w.logger, w.cfg.API.BaseURL, w.cfg.Storage.Bucket)
	return processor.Process(ctx, jobID)
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

	for _, job := range jobs {
		logger := w.logger.With("job_id", job.ID)

		if job.ExecutorRef == nil {
			// Job never started execution
			logger.Info("marking job as failed (no executor reference)")
			msg := "worker crashed before job started"
			if err := w.store.UpdateJobStatus(w.ctx, job.ID, "FAILED", &msg); err != nil {
				logger.Error("failed to update orphaned job", "error", err)
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
		case executor.StatusFailed:
			logger.Info("job failed (recovery)")
			job.Status = "FAILED"
			finishedAt := time.Now()
			job.FinishedAt = &finishedAt
			msg := "executor reported failure"
			job.StatusMessage = &msg
		case executor.StatusUnknown:
			logger.Info("job executor not found (recovery)")
			job.Status = "FAILED"
			finishedAt := time.Now()
			job.FinishedAt = &finishedAt
			msg := "executor service not found after worker restart"
			job.StatusMessage = &msg
		default:
			// Still running, leave it
			logger.Info("job still running (recovery)", "status", status)
			continue
		}

		if err := w.store.UpdateJob(w.ctx, job); err != nil {
			logger.Error("failed to update recovered job", "error", err)
		}
	}

	return nil
}
