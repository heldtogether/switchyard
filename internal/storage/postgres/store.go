package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/domain"
	_ "github.com/lib/pq"
)

// Store handles database operations
type Store struct {
	db *sql.DB
}

// New creates a new Store
func New(dbURL string) (*Store, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateJob inserts a new job
func (s *Store) CreateJob(ctx context.Context, job *domain.Job) error {
	query := `
		INSERT INTO jobs (
			id, run_id, name, created_by, status, image, image_digest, command, env,
			cpu_limit, memory_limit, timeout_seconds, outputs,
			executor, metadata, registry_secret_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING created_at, updated_at
	`

	commandJSON, _ := json.Marshal(job.Command)
	envJSON, _ := json.Marshal(job.Env)
	outputsJSON, _ := json.Marshal(job.Outputs)
	metadataJSON, _ := json.Marshal(job.Metadata)

	err := s.db.QueryRowContext(ctx, query,
		job.ID, job.RunID, job.Name, job.CreatedBy, job.Status, job.Image, job.ImageDigest,
		commandJSON, envJSON, job.CPULimit, job.MemoryLimit, job.TimeoutSecs,
		outputsJSON, job.Executor, metadataJSON, job.RegistrySecretID,
	).Scan(&job.CreatedAt, &job.UpdatedAt)

	return err
}

// GetJob retrieves a job by ID
func (s *Store) GetJob(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	query := `
		SELECT id, run_id, name, created_at, updated_at, created_by, status, status_message,
		       image, image_digest, command, env, cpu_limit, memory_limit, timeout_seconds,
		       outputs, started_at, finished_at, exit_code, artefact_prefix, log_object_key,
		       executor, executor_ref, executor_metadata, registry_secret_id, metadata
		FROM jobs WHERE id = $1
	`

	job := &domain.Job{}
	var commandJSON, envJSON, outputsJSON, execMetadataJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID, &job.RunID, &job.Name, &job.CreatedAt, &job.UpdatedAt, &job.CreatedBy,
		&job.Status, &job.StatusMessage, &job.Image, &job.ImageDigest,
		&commandJSON, &envJSON, &job.CPULimit, &job.MemoryLimit, &job.TimeoutSecs,
		&outputsJSON, &job.StartedAt, &job.FinishedAt, &job.ExitCode,
		&job.ArtefactPrefix, &job.LogObjectKey, &job.Executor, &job.ExecutorRef,
		&execMetadataJSON, &job.RegistrySecretID, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found")
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	json.Unmarshal(commandJSON, &job.Command)
	json.Unmarshal(envJSON, &job.Env)
	json.Unmarshal(outputsJSON, &job.Outputs)
	json.Unmarshal(execMetadataJSON, &job.ExecutorMetadata)
	json.Unmarshal(metadataJSON, &job.Metadata)

	return job, nil
}

// UpdateJobStatus updates a job's status and related fields
func (s *Store) UpdateJobStatus(ctx context.Context, id uuid.UUID, status domain.JobStatus, message *string) error {
	query := `UPDATE jobs SET status = $1, status_message = $2 WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, status, message, id)
	return err
}

// UpdateJob updates a job (for worker updates)
func (s *Store) UpdateJob(ctx context.Context, job *domain.Job) error {
	query := `
		UPDATE jobs SET
			status = $1, status_message = $2, started_at = $3, finished_at = $4,
			exit_code = $5, artefact_prefix = $6, log_object_key = $7,
			executor_ref = $8, executor_metadata = $9, image_digest = $10
		WHERE id = $11
	`

	execMetadataJSON, _ := json.Marshal(job.ExecutorMetadata)

	_, err := s.db.ExecContext(ctx, query,
		job.Status, job.StatusMessage, job.StartedAt, job.FinishedAt,
		job.ExitCode, job.ArtefactPrefix, job.LogObjectKey,
		job.ExecutorRef, execMetadataJSON, job.ImageDigest, job.ID,
	)

	return err
}

// ListJobs lists jobs with filters
func (s *Store) ListJobs(ctx context.Context, runID *uuid.UUID, status *domain.JobStatus, createdBy *string, limit, offset int) ([]*domain.Job, error) {
	query := `
		SELECT id, run_id, name, created_at, updated_at, created_by, status, status_message,
		       image, image_digest, command, env, cpu_limit, memory_limit, timeout_seconds,
		       outputs, started_at, finished_at, exit_code, artefact_prefix, log_object_key,
		       executor, executor_ref, executor_metadata, registry_secret_id, metadata
		FROM jobs
		WHERE ($1::UUID IS NULL OR run_id = $1)
		  AND ($2::job_status IS NULL OR status = $2)
		  AND ($3::VARCHAR IS NULL OR created_by = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
	`

	rows, err := s.db.QueryContext(ctx, query, runID, status, createdBy, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		job := &domain.Job{}
		var commandJSON, envJSON, outputsJSON, execMetadataJSON, metadataJSON []byte

		err := rows.Scan(
			&job.ID, &job.RunID, &job.Name, &job.CreatedAt, &job.UpdatedAt, &job.CreatedBy,
			&job.Status, &job.StatusMessage, &job.Image, &job.ImageDigest,
			&commandJSON, &envJSON, &job.CPULimit, &job.MemoryLimit, &job.TimeoutSecs,
			&outputsJSON, &job.StartedAt, &job.FinishedAt, &job.ExitCode,
			&job.ArtefactPrefix, &job.LogObjectKey, &job.Executor, &job.ExecutorRef,
			&execMetadataJSON, &job.RegistrySecretID, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(commandJSON, &job.Command)
		json.Unmarshal(envJSON, &job.Env)
		json.Unmarshal(outputsJSON, &job.Outputs)
		json.Unmarshal(execMetadataJSON, &job.ExecutorMetadata)
		json.Unmarshal(metadataJSON, &job.Metadata)

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// SaveArtefacts inserts artefacts for a job
func (s *Store) SaveArtefacts(ctx context.Context, jobID uuid.UUID, artefacts []domain.Artefact) error {
	if len(artefacts) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO artefacts (job_id, path, object_key, size_bytes, content_type)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (job_id, path) DO UPDATE
		SET object_key = EXCLUDED.object_key,
		    size_bytes = EXCLUDED.size_bytes,
		    content_type = EXCLUDED.content_type
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, art := range artefacts {
		_, err := stmt.ExecContext(ctx, jobID, art.Path, art.ObjectKey, art.SizeBytes, art.ContentType)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetArtefacts retrieves artefacts for a job
func (s *Store) GetArtefacts(ctx context.Context, jobID uuid.UUID) ([]domain.Artefact, error) {
	query := `
		SELECT id, job_id, path, object_key, size_bytes, content_type, created_at
		FROM artefacts
		WHERE job_id = $1
		ORDER BY path
	`

	rows, err := s.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artefacts []domain.Artefact
	for rows.Next() {
		var art domain.Artefact
		err := rows.Scan(&art.ID, &art.JobID, &art.Path, &art.ObjectKey, &art.SizeBytes, &art.ContentType, &art.CreatedAt)
		if err != nil {
			return nil, err
		}
		artefacts = append(artefacts, art)
	}

	return artefacts, rows.Err()
}

// GetRunningJobs retrieves all jobs in RUNNING status (for recovery)
func (s *Store) GetRunningJobs(ctx context.Context) ([]*domain.Job, error) {
	status := domain.JobStatusRunning
	return s.ListJobs(ctx, nil, &status, nil, 1000, 0)
}

// SetConnPoolLimits configures connection pool
func (s *Store) SetConnPoolLimits(maxOpen, maxIdle int, maxLifetime time.Duration) {
	s.db.SetMaxOpenConns(maxOpen)
	s.db.SetMaxIdleConns(maxIdle)
	s.db.SetConnMaxLifetime(maxLifetime)
}
