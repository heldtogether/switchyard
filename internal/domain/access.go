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
