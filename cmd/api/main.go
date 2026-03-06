package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/heldtogether/switchyard/internal/api"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/executor"
	dockerexec "github.com/heldtogether/switchyard/internal/executor/docker"
	"github.com/heldtogether/switchyard/internal/registrysecrets"
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

	if err := runMigrationsWithRetry(logger, cfg.Database.URL, "migrations"); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	if cfg.Queue.Type == "rabbitmq" {
		if err := waitForRabbitMQ(logger, cfg.Queue.URL); err != nil {
			logger.Error("rabbitmq not ready", "error", err)
			os.Exit(1)
		}
	}

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

	// Initialize queue
	logger.Info("connecting to queue", "type", cfg.Queue.Type)
	var producer queue.Producer
	switch cfg.Queue.Type {
	case "rabbitmq":
		producer, err = queue.NewRabbitPublisher(cfg.Queue.URL, cfg.Queue.Exchange)
	case "redis":
		producer, err = queue.NewRedis(cfg.Queue.URL, cfg.Queue.QueueName)
	default:
		logger.Error("unsupported queue type", "type", cfg.Queue.Type)
		os.Exit(1)
	}
	if err != nil {
		logger.Error("failed to connect to queue", "error", err)
		os.Exit(1)
	}
	defer producer.Close()
	logger.Info("queue connected")

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
			cfg.Executor.Docker.DockerHost,
			cfg.Executor.Docker.NFSBasePath,
			cfg.Executor.Docker.NetworkIsolated,
		)
		if err != nil {
			logger.Error("failed to create docker executor", "error", err)
			os.Exit(1)
		}
	case "kube":
		logger.Error("executor type kube is not implemented yet")
		os.Exit(1)
	default:
		logger.Error("unsupported executor type", "type", cfg.Executor.Type)
		os.Exit(1)
	}
	logger.Info("executor initialized")

	// Build base URL
	baseURL := fmt.Sprintf("http://%s:%d", cfg.API.Host, cfg.API.Port)
	secretCodec, err := registrysecrets.NewCodec(cfg.API.RegistrySecrets.Encryption)
	if err != nil {
		logger.Error("failed to initialize registry secret encryption", "error", err)
		os.Exit(1)
	}

	// Create API
	apiInstance := api.New(cfg, store, producer, s3Store, exec, logger, baseURL, secretCodec)

	// Create and start server
	server := api.NewServer(cfg, apiInstance, logger)
	if err := server.Start(); err != nil {
		logger.Error("failed to start server", "error", err)
		os.Exit(1)
	}

	// Start node reaper
	reaperCtx, reaperCancel := context.WithCancel(context.Background())
	defer reaperCancel()
	apiInstance.StartNodeReaper(reaperCtx)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	logger.Info("api server running", "addr", fmt.Sprintf("%s:%d", cfg.API.Host, cfg.API.Port))
	<-sigChan

	reaperCancel()

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

func runMigrationsWithRetry(logger *slog.Logger, dbURL, migrationsDir string) error {
	const maxAttempts = 30
	const maxDelay = 10 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Info("running migrations", "attempt", attempt, "max_attempts", maxAttempts)
		if err := runMigrations(dbURL, migrationsDir); err == nil {
			return nil
		} else {
			lastErr = err
			logger.Warn("migration attempt failed", "error", err)
		}

		if attempt == maxAttempts {
			break
		}

		sleep := time.Duration(attempt) * time.Second
		if sleep > maxDelay {
			sleep = maxDelay
		}
		time.Sleep(sleep)
	}

	return fmt.Errorf("migrations failed after %d attempts: %w", maxAttempts, lastErr)
}

func runMigrations(dbURL, migrationsDir string) error {
	absDir, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("resolve migrations dir: %w", err)
	}
	sourceURL := fmt.Sprintf("file://%s", absDir)

	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	err = m.Up()
	if err != nil && errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("migration up failed: %w", err)
	}
	return nil
}

func waitForRabbitMQ(logger *slog.Logger, url string) error {
	const maxAttempts = 30
	const maxDelay = 10 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Info("waiting for rabbitmq", "attempt", attempt, "max_attempts", maxAttempts)
		conn, err := amqp.Dial(url)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err

		if attempt == maxAttempts {
			break
		}

		sleep := time.Duration(attempt) * time.Second
		if sleep > maxDelay {
			sleep = maxDelay
		}
		time.Sleep(sleep)
	}

	return fmt.Errorf("rabbitmq not reachable after %d attempts: %w", maxAttempts, lastErr)
}
