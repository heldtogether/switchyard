package domain

import (
	"time"

	"github.com/google/uuid"
)

type PromotionChannel string

const (
	PromotionChannelDev       PromotionChannel = "dev"
	PromotionChannelStaging   PromotionChannel = "staging"
	PromotionChannelProd      PromotionChannel = "prod"
	PromotionChannelValidated PromotionChannel = "validated"
)

func (c PromotionChannel) Valid() bool {
	switch c {
	case PromotionChannelDev, PromotionChannelStaging, PromotionChannelProd, PromotionChannelValidated:
		return true
	default:
		return false
	}
}

type PromotionArtefactRef struct {
	LogicalKey  string    `json:"logical_key"`
	JobID       uuid.UUID `json:"job_id"`
	Path        string    `json:"path"`
	ObjectKey   string    `json:"object_key"`
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type,omitempty"`
}

type PromotionEvent struct {
	ID                    uuid.UUID              `json:"id"`
	WorkspaceID           uuid.UUID              `json:"workspace_id"`
	ProjectID             uuid.UUID              `json:"project_id"`
	Channel               PromotionChannel       `json:"channel"`
	RunID                 uuid.UUID              `json:"run_id"`
	Note                  *string                `json:"note,omitempty"`
	PromotedBy            string                 `json:"promoted_by"`
	PromotedByPrincipalID *uuid.UUID             `json:"promoted_by_principal_id,omitempty"`
	CreatedAt             time.Time              `json:"created_at"`
	Artefacts             []PromotionArtefactRef `json:"artefacts,omitempty"`
}

type CurrentChannelPromotion struct {
	ProjectID uuid.UUID        `json:"project_id"`
	Channel   PromotionChannel `json:"channel"`
	Event     PromotionEvent   `json:"event"`
}

type PromotedArtefactResolution struct {
	Channel              PromotionChannel `json:"channel"`
	LogicalKey           string           `json:"logical_key"`
	PromotionEventID     uuid.UUID        `json:"promotion_event_id"`
	RunID                uuid.UUID        `json:"run_id"`
	JobID                uuid.UUID        `json:"job_id"`
	Path                 string           `json:"path"`
	ObjectKey            string           `json:"object_key"`
	SizeBytes            int64            `json:"size_bytes"`
	ContentType          string           `json:"content_type,omitempty"`
	PromotedAt           time.Time        `json:"promoted_at"`
	PromotedBy           string           `json:"promoted_by"`
	DownloadURL          string           `json:"download_url"`
	DownloadURLExpiresAt time.Time        `json:"download_url_expires_at"`
}
