package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetEnvFromFile_TrimsNewlinesOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("secret\r\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	if err := os.Setenv("TEST_SECRET_FILE", path); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TEST_SECRET_FILE") })

	got := getEnvFromFile("TEST_SECRET_FILE")
	if got != "secret" {
		t.Fatalf("expected trimmed secret, got %q", got)
	}
}

func TestGetEnvFromFile_PreservesSpaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte(" secret \n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	if err := os.Setenv("TEST_SECRET_FILE_SPACES", path); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TEST_SECRET_FILE_SPACES") })

	got := getEnvFromFile("TEST_SECRET_FILE_SPACES")
	if got != " secret " {
		t.Fatalf("expected preserved spaces, got %q", got)
	}
}

func TestGetEnvFromFile_MissingFile(t *testing.T) {
	if err := os.Setenv("TEST_SECRET_FILE_MISSING", "/nope/missing"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TEST_SECRET_FILE_MISSING") })

	got := getEnvFromFile("TEST_SECRET_FILE_MISSING")
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestReplacePasswordInURL(t *testing.T) {
	got := replacePasswordInURL("postgres://user:old@localhost:5432/db?sslmode=disable", "p@ss/word")
	want := "postgres://user:p%40ss%2Fword@localhost:5432/db?sslmode=disable"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestReplacePasswordInURL_NoUserInfo(t *testing.T) {
	original := "postgres://localhost:5432/db?sslmode=disable"
	got := replacePasswordInURL(original, "new")
	if got != original {
		t.Fatalf("expected unchanged url, got %q", got)
	}
}

func TestAuthNormalizeLegacyValues(t *testing.T) {
	auth := AuthConfig{
		Enabled: true,
		APIKey:  "secret",
	}

	auth.Normalize()

	if auth.Mode != "api_key" {
		t.Fatalf("expected mode api_key, got %q", auth.Mode)
	}
	if !auth.APIKeyAuth.Enabled {
		t.Fatalf("expected api key auth enabled")
	}
	if auth.APIKeyAuth.Key != "secret" {
		t.Fatalf("expected api key auth key to be copied from legacy field")
	}
}

func TestOIDCValidate(t *testing.T) {
	oidc := OIDCAuthConfig{
		Enabled:           true,
		IssuerURL:         "https://example.com",
		ClientID:          "client-id",
		ClientSecret:      "secret",
		RedirectURL:       "https://api.example.com/v1/auth/callback",
		SessionSigningKey: "super-secret",
		BearerTokenTTL:    15 * time.Minute,
		Cookie: OIDCCookieConfig{
			SameSite: "lax",
			TTL:      time.Hour,
		},
	}
	if err := oidc.Validate(); err != nil {
		t.Fatalf("expected valid OIDC config, got %v", err)
	}
}

func TestRegistrySecretEncryptionValidate(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))

	cfg := RegistrySecretEncryptionConfig{
		Enabled:       true,
		ActiveKeyID:   "key-1",
		ActiveKey:     key,
		PreviousKeyID: "key-0",
		PreviousKey:   key,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid encryption config, got %v", err)
	}
}

func TestRegistrySecretEncryptionValidate_InvalidKeyLength(t *testing.T) {
	cfg := RegistrySecretEncryptionConfig{
		Enabled:     true,
		ActiveKeyID: "key-1",
		ActiveKey:   base64.StdEncoding.EncodeToString([]byte("short")),
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected invalid key length error")
	}
}

func TestValidate_BillingDefaults(t *testing.T) {
	cfg := &Config{
		API: APIConfig{
			Port: 8080,
			Auth: AuthConfig{
				Mode: "disabled",
			},
		},
		Database: DatabaseConfig{
			URL: "postgres://u:p@localhost:5432/db?sslmode=disable",
		},
		Queue: QueueConfig{
			Type:            "redis",
			URL:             "redis://localhost:6379/0",
			RequeueBatch:    0,
			RequeueInterval: 0,
		},
		Storage: StorageConfig{
			Type:      "s3",
			Endpoint:  "http://minio:9000",
			Bucket:    "bucket",
			AccessKey: "key",
			SecretKey: "secret",
		},
		Executor: ExecutorConfig{
			Type: "docker",
			Docker: DockerConfig{
				NFSBasePath: "/tmp",
				DockerHost:  "unix:///var/run/docker.sock",
			},
		},
		Worker: WorkerConfig{
			Concurrency: 1,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to validate, got: %v", err)
	}
	if cfg.Billing.UsageSampleInterval == 0 {
		t.Fatalf("expected billing usage sample interval default")
	}
	if cfg.Billing.Stripe.Meters.CPUSeconds == "" || cfg.Billing.Stripe.Meters.MemoryGBSeconds == "" {
		t.Fatalf("expected default billing meter names")
	}
}

func TestValidate_BillingEnabledRequiresPricing(t *testing.T) {
	cfg := &Config{
		API: APIConfig{
			Port: 8080,
			Auth: AuthConfig{
				Mode: "disabled",
			},
		},
		Database: DatabaseConfig{
			URL: "postgres://u:p@localhost:5432/db?sslmode=disable",
		},
		Queue: QueueConfig{
			Type: "redis",
			URL:  "redis://localhost:6379/0",
		},
		Storage: StorageConfig{
			Type:      "s3",
			Endpoint:  "http://minio:9000",
			Bucket:    "bucket",
			AccessKey: "key",
			SecretKey: "secret",
		},
		Executor: ExecutorConfig{
			Type: "docker",
			Docker: DockerConfig{
				NFSBasePath: "/tmp",
				DockerHost:  "unix:///var/run/docker.sock",
			},
		},
		Worker: WorkerConfig{
			Concurrency: 1,
		},
		Billing: BillingConfig{
			Enabled: true,
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error when billing enabled without pricing")
	}
}
