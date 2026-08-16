package models

import (
	"time"

	"gorm.io/gorm"
)

// AuthContext stores authentication credentials (cookies, headers, tokens)
// scoped to a specific target and user for authorized security testing.
// This enables multi-account testing scenarios like IDOR, BOLA, and BFLA.
// Sensitive fields MUST be encrypted before storage.
type AuthContext struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// TargetID links this auth context to a specific target
	TargetID uint    `gorm:"not null;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	// UserID links this auth context to the user who created it
	UserID uint    `gorm:"not null;index" json:"user_id"`
	User   *User   `gorm:"foreignKey:UserID" json:"-"`

	// Name is a human-readable identifier for this context (e.g., "Admin Account", "User A")
	Name string `gorm:"size:128;not null" json:"name"`

	// Description provides additional context about this authentication scenario
	Description string `gorm:"type:text" json:"description"`

	// IsActive indicates whether this context is currently active for testing
	IsActive bool `gorm:"default:true;index" json:"is_active"`

	// Authentication Type: 'cookie', 'header', 'token', 'basic', 'oauth2'
	AuthType string `gorm:"size:32;not null;index" json:"auth_type"`

	// EncryptedCredentials stores encrypted sensitive data (JSON format)
	// Structure depends on AuthType:
	// - cookie: {"cookies": [{"name": "...", "value": "...", "domain": "..."}]}
	// - header: {"headers": {"Authorization": "Bearer ...", "X-Custom-Header": "..."}}
	// - token: {"token": "...", "token_type": "Bearer", "expires_at": "..."}
	// - basic: {"username": "...", "password": "..."} // encrypted
	// - oauth2: {"access_token": "...", "refresh_token": "...", "client_id": "..."}
	EncryptedCredentials string `gorm:"type:text;not null" json:"-"`

	// Non-sensitive metadata for quick reference (NOT encrypted)
	// e.g., username, email, role name (if safe to store plain)
	Metadata string `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	// ExpiresAt is optional expiration time for temporary credentials
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// LastUsedAt tracks when this context was last used in a test
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`

	// UsageCount tracks how many times this context has been used
	UsageCount int `gorm:"default:0" json:"usage_count"`

	// CreatedByUserID links to the admin who created this (if different from UserID)
	CreatedByUserID *uint `gorm:"index" json:"created_by_user_id"`
	CreatedByUser   *User `gorm:"foreignKey:CreatedByUserID" json:"-"`
}

// TableName specifies the table name for AuthContext
func (AuthContext) TableName() string {
	return "auth_contexts"
}
