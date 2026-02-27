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

	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/executor"
	dockerexec "github.com/heldtogether/switchyard/internal/executor/docker"
	"github.com/heldtogether/switchyard/internal/executor/swarm"
	"github.com/heldtogether/switchyard/internal/storage/objectstore"
	"github.com/heldtogether/switchyard/internal/storage/postgres"
	"github.com/heldtogether/switchyard/internal/storage/queue"
	"github.com/heldtogether/switchyard/internal/worker"
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

	// Check NFS mount
	if err := cfg.CheckNFSMount(); err != nil {
		log.Fatalf("NFS mount check failed: %v", err)
	}

	// Setup logger
	logger := setupLogger(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("switchyard worker starting", "version", Version)

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

	// Detect node info and GPU count
	nodeID := cfg.Worker.NodeID
	hostname := ""
	if nodeID == "" {
		dockerHost := cfg.Executor.Swarm.DockerHost
		if cfg.Executor.Type == "docker" {
			dockerHost = cfg.Executor.Docker.DockerHost
		}
		detectedNodeID, detectedHostname, detectErr := worker.DetectNodeInfo(context.Background(), dockerHost)
		if detectErr != nil {
			logger.Warn("failed to detect node id, using hostname fallback", "error", detectErr)
			hostname = detectedHostname
		} else {
			nodeID = detectedNodeID
			hostname = detectedHostname
		}
	}
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	if nodeID == "" {
		nodeID = hostname
	}

	gpuTotal := cfg.Worker.GPUCount
	if gpuTotal <= 0 {
		gpuTotal = worker.DetectGPUCount(context.Background())
	}

	// Initialize queue
	logger.Info("connecting to queue", "type", cfg.Queue.Type)
	var consumer queue.Consumer
	switch cfg.Queue.Type {
	case "rabbitmq":
		queueName := cfg.Queue.QueueName
		if nodeID != "" {
			queueName = fmt.Sprintf("%s.%s", cfg.Queue.QueueName, nodeID)
		}
		consumer, err = queue.NewRabbitConsumer(cfg.Queue.URL, cfg.Queue.Exchange, cfg.Queue.DelayExchange, queueName, gpuTotal, cfg.Worker.Concurrency, cfg.Queue.TaskTimeout, cfg.Queue.MaxPriority)
	case "redis":
		consumer, err = queue.NewRedis(cfg.Queue.URL, cfg.Queue.QueueName)
	default:
		logger.Error("unsupported queue type", "type", cfg.Queue.Type)
		os.Exit(1)
	}
	if err != nil {
		logger.Error("failed to connect to queue", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()
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

	// Initialize executor
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
	case "swarm":
		exec, err = swarm.New(
			cfg.Executor.Swarm.DockerHost,
			cfg.Executor.Swarm.NFSBasePath,
			cfg.Executor.Swarm.NetworkIsolated,
		)
		if err != nil {
			logger.Error("failed to create swarm executor", "error", err)
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

	// Create API client for worker callbacks
	apiBaseURL := cfg.API.BaseURL
	if apiBaseURL == "" {
		apiBaseURL = fmt.Sprintf("http://%s:%d", cfg.API.Host, cfg.API.Port)
	}
	apiClient := worker.NewAPIClient(apiBaseURL, cfg.API.Auth.APIKey)

	// Create worker
	w := worker.New(cfg, consumer, store, exec, s3Store, logger, apiClient, nodeID, hostname, gpuTotal)

	// Start worker
	if err := w.Start(); err != nil {
		logger.Error("failed to start worker", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	logger.Info("worker running, press Ctrl+C to stop")
	<-sigChan

	logger.Info("received shutdown signal")
	if err := w.Stop(); err != nil {
		logger.Error("error during shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("worker stopped cleanly")
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
