package registrysecrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/heldtogether/switchyard/internal/config"
	"github.com/heldtogether/switchyard/internal/domain"
)

const (
	ciphertextPrefix = "aead:v1"
	nonceSize        = 12
)

type Codec struct {
	enabled     bool
	activeKeyID string
	activeKey   []byte
	keys        map[string][]byte
}

func NewCodec(cfg config.RegistrySecretEncryptionConfig) (*Codec, error) {
	if !cfg.Enabled {
		return &Codec{enabled: false}, nil
	}

	activeKey, err := decodeKey(cfg.ActiveKey)
	if err != nil {
		return nil, fmt.Errorf("invalid active key: %w", err)
	}
	if strings.TrimSpace(cfg.ActiveKeyID) == "" {
		return nil, errors.New("active key id is required")
	}

	keys := map[string][]byte{cfg.ActiveKeyID: activeKey}
	if strings.TrimSpace(cfg.PreviousKey) != "" || strings.TrimSpace(cfg.PreviousKeyID) != "" {
		if strings.TrimSpace(cfg.PreviousKeyID) == "" {
			return nil, errors.New("previous key id is required when previous key is set")
		}
		prevKey, err := decodeKey(cfg.PreviousKey)
		if err != nil {
			return nil, fmt.Errorf("invalid previous key: %w", err)
		}
		keys[cfg.PreviousKeyID] = prevKey
	}

	return &Codec{
		enabled:     true,
		activeKeyID: cfg.ActiveKeyID,
		activeKey:   activeKey,
		keys:        keys,
	}, nil
}

func (c *Codec) Enabled() bool {
	return c != nil && c.enabled
}

func (c *Codec) ActiveKeyID() string {
	if c == nil {
		return ""
	}
	return c.activeKeyID
}

func RegistrySecretAAD(workspaceID uuid.UUID, host, username string) string {
	return fmt.Sprintf("workspace=%s|host=%s|username=%s", workspaceID.String(), strings.ToLower(strings.TrimSpace(host)), strings.TrimSpace(username))
}

func (c *Codec) Encrypt(workspaceID uuid.UUID, host, username, plaintext string) (string, string, *string, error) {
	if c == nil || !c.enabled {
		return plaintext, domain.RegistrySecretEncodingPlain, nil, nil
	}

	block, err := aes.NewCipher(c.activeKey)
	if err != nil {
		return "", "", nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", "", nil, fmt.Errorf("read nonce: %w", err)
	}

	aad := []byte(RegistrySecretAAD(workspaceID, host, username))
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), aad)
	encoded := ciphertextPrefix + ":" + base64.RawStdEncoding.EncodeToString(nonce) + ":" + base64.RawStdEncoding.EncodeToString(ciphertext)
	keyID := c.activeKeyID
	return encoded, domain.RegistrySecretEncodingAEADV1, &keyID, nil
}

func (c *Codec) Decrypt(secret domain.RegistrySecret) (string, error) {
	if secret.SecretEncoding == "" || secret.SecretEncoding == domain.RegistrySecretEncodingPlain {
		return secret.PasswordEncrypted, nil
	}

	if c == nil || !c.enabled {
		return "", errors.New("registry secret is encrypted but encryption is not enabled")
	}
	if secret.SecretEncoding != domain.RegistrySecretEncodingAEADV1 {
		return "", fmt.Errorf("unsupported registry secret encoding: %s", secret.SecretEncoding)
	}
	if secret.SecretKeyID == nil || strings.TrimSpace(*secret.SecretKeyID) == "" {
		return "", errors.New("encrypted registry secret is missing key id")
	}

	key, ok := c.keys[*secret.SecretKeyID]
	if !ok {
		return "", fmt.Errorf("no decryption key configured for key id %q", *secret.SecretKeyID)
	}

	parts := strings.Split(secret.PasswordEncrypted, ":")
	if len(parts) != 4 || parts[0] != "aead" || parts[1] != "v1" {
		return "", errors.New("invalid encrypted payload format")
	}

	nonce, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	aad := []byte(RegistrySecretAAD(secret.WorkspaceID, secret.Host, secret.Username))
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", fmt.Errorf("decrypt payload: %w", err)
	}
	return string(plaintext), nil
}

func decodeKey(encoded string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, err
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("expected 32-byte key, got %d bytes", len(decoded))
	}
	return decoded, nil
}
