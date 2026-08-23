package models

import (
	"time"
	"gorm.io/gorm"
)

// AuthContextType defines the type of authentication context
type AuthContextType string

const (
	AuthContextCookie  AuthContextType = "cookie"
	AuthContextHeader  AuthContextType = "header"
	AuthContextToken   AuthContextType = "token"
	AuthContextSession AuthContextType = "session"
)

// AuthContext stores authentication credentials for authorized testing (IDOR/BOLA)
// All sensitive fields should be encrypted before storage (handled by service layer)
type AuthContext struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	TargetID    uint            `gorm:"not null;index" json:"target_id"`
	UserID      uint            `gorm:"not null;index" json:"user_id"` // Owner of this context
	Name        string          `gorm:"size:100;not null" json:"name"` // e.g., "Admin Session", "User A Cookie"
	ContextType AuthContextType `gorm:"type:varchar(20);not null" json:"context_type"`
	
	// Encrypted storage for sensitive data
	// Service layer must encrypt/decrypt these fields
	KeyName   string `gorm:"size:100" json:"key_name"`   // e.g., "Authorization", "session_id"
	Value     string `gorm:"type:text;not null" json:"value"` // The actual secret (encrypted)
	Domain    string `gorm:"size:255" json:"domain"`     // Optional: scope domain
	Path      string `gorm:"size:255;default:/" json:"path"`
	ExpiresAt *time.Time `json:"expires_at"`
	
	// Metadata
	Description string `gorm:"type:text" json:"description"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
	
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (AuthContext) TableName() string {
	return "auth_contexts"
}
