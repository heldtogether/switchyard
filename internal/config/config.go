package config

import (
	"fmt"
	urlpkg "net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	API       APIConfig       `yaml:"api"`
	Worker    WorkerConfig    `yaml:"worker"`
	Database  DatabaseConfig  `yaml:"database"`
	Queue     QueueConfig     `yaml:"queue"`
	Storage   StorageConfig   `yaml:"storage"`
	Executor  ExecutorConfig  `yaml:"executor"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// APIConfig holds API server configuration
type APIConfig struct {
	Port               int           `yaml:"port"`
	Host               string        `yaml:"host"`
	BaseURL            string        `yaml:"base_url"`
	ReadTimeout        time.Duration `yaml:"read_timeout"`
	WriteTimeout       time.Duration `yaml:"write_timeout"`
	CORSAllowedOrigins []string      `yaml:"cors_allowed_origins"`
	Auth               AuthConfig    `yaml:"auth"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Mode       string           `yaml:"mode"`    // disabled, api_key, oidc, hybrid
	Enabled    bool             `yaml:"enabled"` // Legacy: maps to api_key mode when true.
	APIKey     string           `yaml:"api_key"` // Legacy: maps to api_key_auth.key.
	APIKeyAuth APIKeyAuthConfig `yaml:"api_key_auth"`
	OIDC       OIDCAuthConfig   `yaml:"oidc"`
}

type APIKeyAuthConfig struct {
	Enabled bool   `yaml:"enabled"`
	Key     string `yaml:"key"`
}

type OIDCAuthConfig struct {
	Enabled            bool             `yaml:"enabled"`
	IssuerURL          string           `yaml:"issuer_url"`
	ClientID           string           `yaml:"client_id"`
	ClientSecret       string           `yaml:"client_secret"`
	RedirectURL        string           `yaml:"redirect_url"`
	LogoutURL          string           `yaml:"logout_url"`
	Scopes             []string         `yaml:"scopes"`
	PostLoginRedirect  string           `yaml:"post_login_redirect"`
	PostLogoutRedirect string           `yaml:"post_logout_redirect"`
	SessionSigningKey  string           `yaml:"session_signing_key"`
	Cookie             OIDCCookieConfig `yaml:"cookie"`
}

type OIDCCookieConfig struct {
	Name     string        `yaml:"name"`
	Domain   string        `yaml:"domain"`
	Secure   bool          `yaml:"secure"`
	SameSite string        `yaml:"same_site"` // lax, strict, none
	TTL      time.Duration `yaml:"ttl"`
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	Concurrency       int           `yaml:"concurrency"`
	PollInterval      time.Duration `yaml:"poll_interval"`
	GracefulShutdown  time.Duration `yaml:"graceful_shutdown"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	NodeID            string        `yaml:"node_id"`
	GPUCount          int           `yaml:"gpu_count"`
	GPUDetectImage    string        `yaml:"gpu_detect_image"`
	RetryBaseDelay    time.Duration `yaml:"retry_base_delay"`
	RetryMaxDelay     time.Duration `yaml:"retry_max_delay"`
	RetryJitter       float64       `yaml:"retry_jitter"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL             string        `yaml:"url"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// QueueConfig holds queue configuration
type QueueConfig struct {
	Type            string        `yaml:"type"` // redis, rabbitmq
	URL             string        `yaml:"url"`
	QueueName       string        `yaml:"queue_name"`
	Exchange        string        `yaml:"exchange"`
	DelayExchange   string        `yaml:"delay_exchange"`
	TaskTimeout     time.Duration `yaml:"task_timeout"`
	MaxPriority     int           `yaml:"max_priority"`
	RequeueInterval time.Duration `yaml:"requeue_interval"`
	RequeueBatch    int           `yaml:"requeue_batch"`
}

// SchedulerConfig holds scheduler/reaper configuration
type SchedulerConfig struct {
	ReaperInterval   time.Duration `yaml:"reaper_interval"`
	HeartbeatTimeout time.Duration `yaml:"heartbeat_timeout"`
}

// StorageConfig holds object storage configuration
type StorageConfig struct {
	Type         string `yaml:"type"` // s3
	Endpoint     string `yaml:"endpoint"`
	AccessKey    string `yaml:"access_key"`
	SecretKey    string `yaml:"secret_key"`
	Bucket       string `yaml:"bucket"`
	Region       string `yaml:"region"`
	UseSSL       bool   `yaml:"use_ssl"`
	CreateBucket bool   `yaml:"create_bucket"` // Auto-create bucket if missing
}

// ExecutorConfig holds executor configuration
type ExecutorConfig struct {
	Type   string       `yaml:"type"` // swarm, docker, kube
	Docker DockerConfig `yaml:"docker"`
	Swarm  SwarmConfig  `yaml:"swarm"`
	Kube   KubeConfig   `yaml:"kube"`
}

// DockerConfig holds Docker executor configuration
type DockerConfig struct {
	// ⚠️ CRITICAL: NFS mount base path
	// Must be accessible on all nodes
	NFSBasePath string `yaml:"nfs_base_path"`

	DockerHost      string                 `yaml:"docker_host"`
	NetworkIsolated bool                   `yaml:"network_isolated"`
	Defaults        ExecutorDefaultsConfig `yaml:"defaults"`
	Cleanup         CleanupConfig          `yaml:"cleanup"`
}

// SwarmConfig holds Docker Swarm executor configuration
type SwarmConfig struct {
	// ⚠️ CRITICAL: NFS mount base path
	// Must be accessible on all Swarm nodes
	NFSBasePath string `yaml:"nfs_base_path"`

	DockerHost      string                 `yaml:"docker_host"`
	NetworkIsolated bool                   `yaml:"network_isolated"`
	Defaults        ExecutorDefaultsConfig `yaml:"defaults"`
	Cleanup         CleanupConfig          `yaml:"cleanup"`
}

// ExecutorDefaultsConfig holds default settings for executor jobs
type ExecutorDefaultsConfig struct {
	Resources   ResourcesConfig `yaml:"resources"`
	Timeout     time.Duration   `yaml:"timeout"`
	Constraints []string        `yaml:"constraints"`
}

// ResourcesConfig holds resource limits
type ResourcesConfig struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}

// CleanupConfig holds cleanup policy configuration
type CleanupConfig struct {
	RemoveOnComplete   bool `yaml:"remove_on_complete"`
	KeepFailedServices bool `yaml:"keep_failed_services"`
}

// KubeConfig holds Kubernetes executor configuration (future)
type KubeConfig struct {
	Kubeconfig string `yaml:"kubeconfig"`
	Namespace  string `yaml:"namespace"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, console
}

// Load reads configuration from a YAML file and applies environment overrides
func Load(path string) (*Config, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&cfg)

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(cfg *Config) {
	// API
	if v := os.Getenv("API_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.API.Port)
	}
	if v := os.Getenv("AUTH_MODE"); v != "" {
		cfg.API.Auth.Mode = v
	}
	if v := os.Getenv("API_KEY"); v != "" {
		cfg.API.Auth.APIKey = v
		cfg.API.Auth.APIKeyAuth.Key = v
	}
	if v := getEnvFromFile("API_KEY_FILE"); v != "" {
		cfg.API.Auth.APIKey = v
		cfg.API.Auth.APIKeyAuth.Key = v
	}
	if v := os.Getenv("OIDC_ISSUER_URL"); v != "" {
		cfg.API.Auth.OIDC.IssuerURL = v
	}
	if v := os.Getenv("OIDC_CLIENT_ID"); v != "" {
		cfg.API.Auth.OIDC.ClientID = v
	}
	if v := os.Getenv("OIDC_CLIENT_SECRET"); v != "" {
		cfg.API.Auth.OIDC.ClientSecret = v
	}
	if v := getEnvFromFile("OIDC_CLIENT_SECRET_FILE"); v != "" {
		cfg.API.Auth.OIDC.ClientSecret = v
	}
	if v := os.Getenv("OIDC_REDIRECT_URL"); v != "" {
		cfg.API.Auth.OIDC.RedirectURL = v
	}
	if v := os.Getenv("OIDC_LOGOUT_URL"); v != "" {
		cfg.API.Auth.OIDC.LogoutURL = v
	}
	if v := os.Getenv("OIDC_SCOPES"); v != "" {
		parts := strings.Split(v, ",")
		scopes := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				scopes = append(scopes, trimmed)
			}
		}
		cfg.API.Auth.OIDC.Scopes = scopes
	}
	if v := os.Getenv("OIDC_POST_LOGIN_REDIRECT"); v != "" {
		cfg.API.Auth.OIDC.PostLoginRedirect = v
	}
	if v := os.Getenv("OIDC_POST_LOGOUT_REDIRECT"); v != "" {
		cfg.API.Auth.OIDC.PostLogoutRedirect = v
	}
	if v := os.Getenv("OIDC_SESSION_SIGNING_KEY"); v != "" {
		cfg.API.Auth.OIDC.SessionSigningKey = v
	}
	if v := getEnvFromFile("OIDC_SESSION_SIGNING_KEY_FILE"); v != "" {
		cfg.API.Auth.OIDC.SessionSigningKey = v
	}
	if v := os.Getenv("OIDC_COOKIE_NAME"); v != "" {
		cfg.API.Auth.OIDC.Cookie.Name = v
	}
	if v := os.Getenv("OIDC_COOKIE_DOMAIN"); v != "" {
		cfg.API.Auth.OIDC.Cookie.Domain = v
	}
	if v := os.Getenv("OIDC_COOKIE_SECURE"); v != "" {
		cfg.API.Auth.OIDC.Cookie.Secure = v == "true" || v == "1"
	}
	if v := os.Getenv("OIDC_COOKIE_SAMESITE"); v != "" {
		cfg.API.Auth.OIDC.Cookie.SameSite = v
	}
	if v := os.Getenv("OIDC_COOKIE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.API.Auth.OIDC.Cookie.TTL = d
		}
	}
	if v := os.Getenv("API_CORS_ALLOWED_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		origins := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		cfg.API.CORSAllowedOrigins = origins
	}

	// Database
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := getEnvFromFile("DATABASE_PASSWORD_FILE"); v != "" {
		// Extract password and rebuild URL
		cfg.Database.URL = replacePasswordInURL(cfg.Database.URL, v)
	}

	// Redis
	if v := os.Getenv("REDIS_URL"); v != "" {
		cfg.Queue.URL = v
	}
	if v := os.Getenv("QUEUE_URL"); v != "" {
		cfg.Queue.URL = v
	}
	if v := os.Getenv("QUEUE_TYPE"); v != "" {
		cfg.Queue.Type = v
	}
	if v := os.Getenv("QUEUE_EXCHANGE"); v != "" {
		cfg.Queue.Exchange = v
	}
	if v := os.Getenv("QUEUE_DELAY_EXCHANGE"); v != "" {
		cfg.Queue.DelayExchange = v
	}
	if v := os.Getenv("QUEUE_NAME"); v != "" {
		cfg.Queue.QueueName = v
	}
	if v := os.Getenv("QUEUE_TASK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Queue.TaskTimeout = d
		}
	}
	if v := os.Getenv("QUEUE_MAX_PRIORITY"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Queue.MaxPriority)
	}
	if v := os.Getenv("QUEUE_REQUEUE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Queue.RequeueInterval = d
		}
	}
	if v := os.Getenv("QUEUE_REQUEUE_BATCH"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Queue.RequeueBatch)
	}

	if v := os.Getenv("SCHEDULER_REAPER_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Scheduler.ReaperInterval = d
		}
	}
	if v := os.Getenv("SCHEDULER_HEARTBEAT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Scheduler.HeartbeatTimeout = d
		}
	}

	// S3
	if v := os.Getenv("S3_ENDPOINT"); v != "" {
		cfg.Storage.Endpoint = v
	}
	if v := os.Getenv("S3_ACCESS_KEY"); v != "" {
		cfg.Storage.AccessKey = v
	}
	if v := getEnvFromFile("S3_ACCESS_KEY_FILE"); v != "" {
		cfg.Storage.AccessKey = v
	}
	if v := os.Getenv("S3_SECRET_KEY"); v != "" {
		cfg.Storage.SecretKey = v
	}
	if v := getEnvFromFile("S3_SECRET_KEY_FILE"); v != "" {
		cfg.Storage.SecretKey = v
	}
	if v := os.Getenv("S3_BUCKET"); v != "" {
		cfg.Storage.Bucket = v
	}
	if v := os.Getenv("S3_REGION"); v != "" {
		cfg.Storage.Region = v
	}

	// Executor
	if v := os.Getenv("EXECUTOR_TYPE"); v != "" {
		cfg.Executor.Type = v
	}
	if v := os.Getenv("EXECUTOR_NFS_BASE"); v != "" {
		cfg.Executor.Docker.NFSBasePath = v
		cfg.Executor.Swarm.NFSBasePath = v
	}
	if v := os.Getenv("DOCKER_HOST"); v != "" {
		cfg.Executor.Docker.DockerHost = v
		cfg.Executor.Swarm.DockerHost = v
	}

	// Worker
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Worker.Concurrency)
	}
	if v := os.Getenv("WORKER_HEARTBEAT_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Worker.HeartbeatInterval = d
		}
	}
	if v := os.Getenv("WORKER_NODE_ID"); v != "" {
		cfg.Worker.NodeID = v
	}
	if v := os.Getenv("WORKER_GPU_COUNT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Worker.GPUCount)
	}
	if v := os.Getenv("WORKER_GPU_DETECT_IMAGE"); v != "" {
		cfg.Worker.GPUDetectImage = v
	}
	if v := os.Getenv("WORKER_RETRY_BASE_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Worker.RetryBaseDelay = d
		}
	}
	if v := os.Getenv("WORKER_RETRY_MAX_DELAY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Worker.RetryMaxDelay = d
		}
	}
	if v := os.Getenv("WORKER_RETRY_JITTER"); v != "" {
		fmt.Sscanf(v, "%f", &cfg.Worker.RetryJitter)
	}

	// Logging
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
}

// getEnvFromFile reads a secret from a Docker secret file
func getEnvFromFile(envVar string) string {
	path := os.Getenv(envVar)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\r\n")
}

// replacePasswordInURL replaces password in a Postgres URL
func replacePasswordInURL(url, password string) string {
	parsed, err := urlpkg.Parse(url)
	if err != nil {
		return url
	}

	if parsed.User == nil {
		return url
	}

	username := parsed.User.Username()
	if username == "" {
		return url
	}

	parsed.User = urlpkg.UserPassword(username, password)
	return parsed.String()
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	c.API.Auth.Normalize()

	// API validation
	if c.API.Port < 1 || c.API.Port > 65535 {
		return fmt.Errorf("invalid API port: %d", c.API.Port)
	}
	switch c.API.Auth.Mode {
	case "disabled":
	case "api_key":
		if !c.API.Auth.APIKeyAuth.Enabled || c.API.Auth.APIKeyAuth.Key == "" {
			return fmt.Errorf("API auth mode is api_key but key is empty")
		}
	case "oidc":
		if err := c.API.Auth.OIDC.Validate(); err != nil {
			return err
		}
	case "hybrid":
		if !c.API.Auth.APIKeyAuth.Enabled || c.API.Auth.APIKeyAuth.Key == "" {
			return fmt.Errorf("API auth mode is hybrid but api_key_auth.key is empty")
		}
		if err := c.API.Auth.OIDC.Validate(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid auth mode: %s (must be 'disabled', 'api_key', 'oidc', or 'hybrid')", c.API.Auth.Mode)
	}

	// Database validation
	if c.Database.URL == "" {
		return fmt.Errorf("database URL is required")
	}

	// Queue validation
	if c.Queue.URL == "" {
		return fmt.Errorf("queue URL is required")
	}
	if c.Queue.Type == "" {
		c.Queue.Type = "redis"
	}
	if c.Queue.Type != "redis" && c.Queue.Type != "rabbitmq" {
		return fmt.Errorf("invalid queue type: %s (must be 'redis' or 'rabbitmq')", c.Queue.Type)
	}
	if c.Queue.Type == "rabbitmq" {
		if c.Queue.Exchange == "" {
			return fmt.Errorf("queue exchange is required for rabbitmq")
		}
		if c.Queue.DelayExchange == "" {
			return fmt.Errorf("queue delay_exchange is required for rabbitmq")
		}
		if c.Queue.QueueName == "" {
			return fmt.Errorf("queue_name is required for rabbitmq")
		}
		if c.Queue.TaskTimeout == 0 {
			return fmt.Errorf("queue task_timeout is required for rabbitmq")
		}
	}
	if c.Queue.RequeueInterval == 0 {
		c.Queue.RequeueInterval = 2 * time.Second
	}
	if c.Queue.RequeueBatch == 0 {
		c.Queue.RequeueBatch = 100
	}

	// Storage validation
	if c.Storage.Type != "s3" {
		return fmt.Errorf("invalid storage type: %s (must be 's3')", c.Storage.Type)
	}
	if c.Storage.Endpoint == "" {
		return fmt.Errorf("storage endpoint is required")
	}
	if c.Storage.Bucket == "" {
		return fmt.Errorf("storage bucket is required")
	}
	if c.Storage.AccessKey == "" || c.Storage.SecretKey == "" {
		return fmt.Errorf("storage credentials are required")
	}

	// Executor validation
	if c.Executor.Type != "swarm" && c.Executor.Type != "kube" && c.Executor.Type != "docker" {
		return fmt.Errorf("invalid executor type: %s (must be 'swarm' or 'kube' or 'docker')", c.Executor.Type)
	}

	if c.Executor.Type == "docker" {
		if c.Executor.Docker.NFSBasePath == "" {
			return fmt.Errorf("docker executor: nfs_base_path is required")
		}
		if c.Executor.Docker.DockerHost == "" {
			return fmt.Errorf("docker executor: docker_host is required")
		}
	}
	if c.Executor.Type == "swarm" {
		if c.Executor.Swarm.NFSBasePath == "" {
			return fmt.Errorf("swarm executor: nfs_base_path is required")
		}
		if c.Executor.Swarm.DockerHost == "" {
			return fmt.Errorf("swarm executor: docker_host is required")
		}
	}

	// Worker validation
	if c.Worker.Concurrency < 1 {
		return fmt.Errorf("worker concurrency must be at least 1")
	}
	if c.Worker.HeartbeatInterval == 0 {
		c.Worker.HeartbeatInterval = 10 * time.Second
	}
	if c.Worker.RetryBaseDelay == 0 {
		c.Worker.RetryBaseDelay = 1 * time.Second
	}
	if c.Worker.RetryMaxDelay == 0 {
		c.Worker.RetryMaxDelay = 30 * time.Second
	}
	if c.Worker.RetryJitter == 0 {
		c.Worker.RetryJitter = 0.2
	}

	if c.Scheduler.ReaperInterval == 0 {
		c.Scheduler.ReaperInterval = 30 * time.Second
	}
	if c.Scheduler.HeartbeatTimeout == 0 {
		c.Scheduler.HeartbeatTimeout = 3 * c.Worker.HeartbeatInterval
	}

	return nil
}

func (a *AuthConfig) Normalize() {
	if a.APIKeyAuth.Key == "" && a.APIKey != "" {
		a.APIKeyAuth.Key = a.APIKey
	}
	if a.APIKey != a.APIKeyAuth.Key {
		a.APIKey = a.APIKeyAuth.Key
	}

	if a.Mode == "" {
		switch {
		case a.Enabled || a.APIKeyAuth.Key != "" || a.APIKeyAuth.Enabled:
			a.Mode = "api_key"
		case a.OIDC.Enabled || a.OIDC.IssuerURL != "":
			a.Mode = "oidc"
		default:
			a.Mode = "disabled"
		}
	}

	switch a.Mode {
	case "api_key":
		a.APIKeyAuth.Enabled = true
		a.OIDC.Enabled = false
	case "oidc":
		a.APIKeyAuth.Enabled = false
		a.OIDC.Enabled = true
	case "hybrid":
		a.APIKeyAuth.Enabled = true
		a.OIDC.Enabled = true
	case "disabled":
		a.APIKeyAuth.Enabled = false
		a.OIDC.Enabled = false
	}

	if len(a.OIDC.Scopes) == 0 {
		a.OIDC.Scopes = []string{"openid", "profile", "email"}
	}
	if a.OIDC.Cookie.Name == "" {
		a.OIDC.Cookie.Name = "switchyard_session"
	}
	if a.OIDC.Cookie.TTL == 0 {
		a.OIDC.Cookie.TTL = 24 * time.Hour
	}
	if a.OIDC.Cookie.SameSite == "" {
		a.OIDC.Cookie.SameSite = "lax"
	}
	if a.OIDC.PostLoginRedirect == "" {
		a.OIDC.PostLoginRedirect = "/"
	}
	if a.OIDC.PostLogoutRedirect == "" {
		a.OIDC.PostLogoutRedirect = "/login"
	}
}

func (o *OIDCAuthConfig) Validate() error {
	if !o.Enabled {
		return nil
	}
	if o.IssuerURL == "" {
		return fmt.Errorf("oidc issuer_url is required")
	}
	if o.ClientID == "" {
		return fmt.Errorf("oidc client_id is required")
	}
	if o.ClientSecret == "" {
		return fmt.Errorf("oidc client_secret is required")
	}
	if o.RedirectURL == "" {
		return fmt.Errorf("oidc redirect_url is required")
	}
	if o.SessionSigningKey == "" {
		return fmt.Errorf("oidc session_signing_key is required")
	}
	if o.Cookie.TTL <= 0 {
		return fmt.Errorf("oidc cookie ttl must be > 0")
	}
	switch strings.ToLower(o.Cookie.SameSite) {
	case "lax", "strict", "none":
	default:
		return fmt.Errorf("oidc cookie same_site must be one of: lax, strict, none")
	}
	return nil
}

// CheckNFSMount verifies that the NFS base path exists and is writable
func (c *Config) CheckNFSMount() error {
	if c.Executor.Type != "swarm" && c.Executor.Type != "docker" {
		return nil
	}

	path := c.Executor.Swarm.NFSBasePath
	if c.Executor.Type == "docker" {
		path = c.Executor.Docker.NFSBasePath
	}

	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("NFS base path does not exist: %s\n"+
				"Please mount your NFS share at this location or update 'executor.swarm.nfs_base_path' in config.yaml", path)
		}
		return fmt.Errorf("failed to stat NFS base path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("NFS base path is not a directory: %s", path)
	}

	// Check if writable (try to create a test file)
	testFile := path + "/.switchyard-test"
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("NFS base path is not writable: %s\nError: %w", path, err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}
