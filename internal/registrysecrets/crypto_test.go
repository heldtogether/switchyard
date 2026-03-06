package registrysecrets

import (
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
)

func TestCodec_EncryptDecrypt(t *testing.T) {
	activeKey := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	codec, err := NewCodec(config.RegistrySecretEncryptionConfig{
		Enabled:     true,
		ActiveKeyID: "key-1",
		ActiveKey:   activeKey,
	})
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}

	workspaceID := uuid.New()
	ciphertext, encoding, keyID, err := codec.Encrypt(workspaceID, "docker.io", "robot", "super-secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encoding != domain.RegistrySecretEncodingAEADV1 {
		t.Fatalf("expected %q, got %q", domain.RegistrySecretEncodingAEADV1, encoding)
	}
	if keyID == nil || *keyID != "key-1" {
		t.Fatalf("unexpected key id: %v", keyID)
	}

	plain, err := codec.Decrypt(domain.RegistrySecret{
		WorkspaceID:       workspaceID,
		Host:              "docker.io",
		Username:          "robot",
		PasswordEncrypted: ciphertext,
		SecretEncoding:    encoding,
		SecretKeyID:       keyID,
	})
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != "super-secret" {
		t.Fatalf("unexpected plaintext: %q", plain)
	}
}

func TestCodec_PlainFallbackWhenDisabled(t *testing.T) {
	codec, err := NewCodec(config.RegistrySecretEncryptionConfig{})
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}

	ciphertext, encoding, keyID, err := codec.Encrypt(uuid.New(), "docker.io", "robot", "plain")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext != "plain" {
		t.Fatalf("expected plain passthrough, got %q", ciphertext)
	}
	if encoding != domain.RegistrySecretEncodingPlain {
		t.Fatalf("expected plain encoding, got %q", encoding)
	}
	if keyID != nil {
		t.Fatalf("expected nil key id")
	}
}
