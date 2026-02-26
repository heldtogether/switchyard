package config

import (
	"os"
	"path/filepath"
	"testing"
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
