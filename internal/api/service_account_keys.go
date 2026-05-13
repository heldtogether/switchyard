package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
)

func generateServiceAccountToken(keyID uuid.UUID) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "swy_sa_" + strings.ReplaceAll(keyID.String(), "-", "") + "_" + randomToken(32)
	}
	return "swy_sa_" + strings.ReplaceAll(keyID.String(), "-", "") + "_" + base64.RawURLEncoding.EncodeToString(b)
}

func hashServiceAccountToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func serviceAccountTokenPrefix(token string) string {
	if len(token) <= 18 {
		return token
	}
	return token[:18]
}
