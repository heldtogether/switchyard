package domain

import (
	"time"

	"github.com/google/uuid"
)

type MemberRole string

const (
	MemberRoleOwner  MemberRole = "owner"
	MemberRoleMember MemberRole = "member"
)

type Principal struct {
	ID          uuid.UUID `json:"id"`
	Subject     string    `json:"subject"`
	Email       *string   `json:"email,omitempty"`
	DisplayName *string   `json:"display_name,omitempty"`
	Provider    *string   `json:"provider,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ServiceAccount struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	PrincipalID uuid.UUID  `json:"principal_id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty"`
	DisabledBy  *string    `json:"disabled_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   string     `json:"created_by"`
	Principal   *Principal `json:"principal,omitempty"`
}

type ServiceAccountKey struct {
	ID               uuid.UUID  `json:"id"`
	ServiceAccountID uuid.UUID  `json:"service_account_id"`
	Name             *string    `json:"name,omitempty"`
	TokenHash        string     `json:"-"`
	TokenPrefix      string     `json:"token_prefix"`
	ExpiresAt        time.Time  `json:"expires_at"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	RevokedBy        *string    `json:"revoked_by,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	CreatedBy        string     `json:"created_by"`
}

type WorkspaceMembership struct {
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	PrincipalID uuid.UUID  `json:"principal_id"`
	Role        MemberRole `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `json:"created_by"`
	Workspace   *Workspace `json:"workspace,omitempty"`
	Principal   *Principal `json:"principal,omitempty"`
}

type ProjectMembership struct {
	ProjectID   uuid.UUID  `json:"project_id"`
	PrincipalID uuid.UUID  `json:"principal_id"`
	Role        MemberRole `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `json:"created_by"`
	Project     *Project   `json:"project,omitempty"`
	Principal   *Principal `json:"principal,omitempty"`
}

type WorkspaceInvite struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	Email       string     `json:"email"`
	Role        MemberRole `json:"role"`
	TokenHash   string     `json:"-"`
	ExpiresAt   time.Time  `json:"expires_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `json:"created_by"`
}

type ProjectInvite struct {
	ID         uuid.UUID  `json:"id"`
	ProjectID  uuid.UUID  `json:"project_id"`
	Email      string     `json:"email"`
	Role       MemberRole `json:"role"`
	TokenHash  string     `json:"-"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by"`
}
