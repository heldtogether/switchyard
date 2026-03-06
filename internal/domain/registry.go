package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	RegistrySecretEncodingPlain  = "plain"
	RegistrySecretEncodingAEADV1 = "aead_v1"
)

// RegistrySecret holds credentials for private Docker registries
type RegistrySecret struct {
	ID                uuid.UUID `json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	CreatedBy         string    `json:"created_by"`
	WorkspaceID       uuid.UUID `json:"workspace_id"`
	Host              string    `json:"host"` // e.g., "docker.io", "gcr.io", "registry.company.com"
	Username          string    `json:"username"`
	PasswordEncrypted string    `json:"-"` // Never expose in JSON
	SecretEncoding    string    `json:"-"`
	SecretKeyID       *string   `json:"-"`
	Active            bool      `json:"active"`
	DeactivatedAt     *time.Time
	DeactivatedBy     *string
	RotatedFromID     *uuid.UUID
}
