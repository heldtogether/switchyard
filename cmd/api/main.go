package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/heldtogether/switchyard/internal/api"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/executor"
	dockerexec "github.com/heldtogether/switchyard/internal/executor/docker"
	swarmexec "github.com/heldtogether/switchyard/internal/executor/swarm"
	"github.com/heldtogether/switchyard/internal/storage/objectstore"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/storage/queue"
)

var Version = "dev"

func main() {
	// Parse flags
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Override from environment if present
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		*configPath = envPath
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	// Setup logger
	logger := setupLogger(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("switchyard api starting", "version", Version)

	// Initialize Postgres
	logger.Info("connecting to database")
	store, err := postgres.New(cfg.Database.URL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	store.SetConnPoolLimits(
		cfg.Database.MaxOpenConns,
		cfg.Database.MaxIdleConns,
		cfg.Database.ConnMaxLifetime,
	)
	logger.Info("database connected")

	// Initialize Redis queue
	logger.Info("connecting to redis")
	redisQueue, err := queue.NewRedis(cfg.Queue.URL, cfg.Queue.QueueName)
	if err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisQueue.Close()
	logger.Info("redis connected")

	// Initialize S3 storage
	logger.Info("connecting to s3 storage")
	s3Store, err := objectstore.NewS3(
		cfg.Storage.Endpoint,
		cfg.Storage.AccessKey,
		cfg.Storage.SecretKey,
		cfg.Storage.Region,
		cfg.Storage.Bucket,
		cfg.Storage.UseSSL,
	)
	if err != nil {
		logger.Error("failed to create s3 client", "error", err)
		os.Exit(1)
	}

	// Create bucket if configured
	if cfg.Storage.CreateBucket {
		logger.Info("ensuring bucket exists", "bucket", cfg.Storage.Bucket)
		if err := s3Store.CreateBucket(context.Background()); err != nil {
			logger.Warn("failed to create bucket (may already exist)", "error", err)
		}
	}
	logger.Info("s3 storage connected")

	// Initialize executor based on config
	logger.Info("initializing executor", "type", cfg.Executor.Type)
	var exec executor.Executor
	switch cfg.Executor.Type {
	case "docker":
		exec, err = dockerexec.New(
			cfg.Executor.Swarm.DockerHost,
			cfg.Executor.Swarm.NFSBasePath,
			cfg.Executor.Swarm.NetworkIsolated,
		)
		if err != nil {
			logger.Error("failed to create docker executor", "error", err)
			os.Exit(1)
		}
	case "swarm":
		exec, err = swarmexec.New(
			cfg.Executor.Swarm.DockerHost,
			cfg.Executor.Swarm.NFSBasePath,
			cfg.Executor.Swarm.NetworkIsolated,
		)
		if err != nil {
			logger.Error("failed to create swarm executor", "error", err)
			os.Exit(1)
		}
	default:
		logger.Error("unsupported executor type", "type", cfg.Executor.Type)
		os.Exit(1)
	}
	logger.Info("executor initialized")

	// Build base URL
	baseURL := fmt.Sprintf("http://%s:%d", cfg.API.Host, cfg.API.Port)

	// Create API
	apiInstance := api.New(cfg, store, redisQueue, s3Store, exec, logger, baseURL)

	// Create and start server
	server := api.NewServer(cfg, apiInstance, logger)
	if err := server.Start(); err != nil {
		logger.Error("failed to start server", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	logger.Info("api server running", "addr", fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port))
	<-sigChan

	logger.Info("received shutdown signal")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Error("error during shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("api server stopped cleanly")
}

// setupLogger creates a structured logger
func setupLogger(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
