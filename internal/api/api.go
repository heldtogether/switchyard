package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/executor"
	"github.com/heldtogether/switchyard/internal/storage/objectstore"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/storage/queue"
)

// API holds the API dependencies
type API struct {
	cfg      *config.Config
	store    *postgres.Store
	queue    *queue.RedisQueue
	storage  *objectstore.S3Store
	executor executor.Executor
	logger   *slog.Logger
	baseURL  string
}

// New creates a new API instance
func New(cfg *config.Config, store *postgres.Store, q *queue.RedisQueue, storage *objectstore.S3Store, exec executor.Executor, logger *slog.Logger, baseURL string) *API {
	return &API{
		cfg:      cfg,
		store:    store,
		queue:    q,
		storage:  storage,
		executor: exec,
		logger:   logger,
		baseURL:  baseURL,
	}
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// validateEnvVars checks that user-provided env vars don't use reserved names.
// Environment variables starting with "SWITCHYARD_" are reserved for system use
// and cannot be set by users.
func validateEnvVars(env map[string]string) error {
	const reservedPrefix = "SWITCHYARD_"

	for key := range env {
		if strings.HasPrefix(key, reservedPrefix) {
			return fmt.Errorf("environment variable '%s' is reserved (variables starting with '%s' are system-managed)", key, reservedPrefix)
		}
	}

	return nil
}
