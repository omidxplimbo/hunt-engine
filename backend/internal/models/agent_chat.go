package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	AgentChatSessionStatusOpen     = "open"
	AgentChatSessionStatusArchived = "archived"

	AgentChatMessageRoleUser      = "user"
	AgentChatMessageRoleAssistant = "assistant"
	AgentChatMessageRoleSystem    = "system"
	AgentChatMessageRoleTool      = "tool"

	AgentChatMessageTypeText       = "text"
	AgentChatMessageTypeActionPlan = "action_plan"
)

// AgentChatSession stores a target-scoped conversational agent session.
// The chat agent is an orchestration/planning layer; execution must go through agent_actions.
type AgentChatSession struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint    `gorm:"not null;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	CreatedByUserID *uint `gorm:"index" json:"created_by_user_id"`
	CreatedByUser   *User `gorm:"foreignKey:CreatedByUserID" json:"-"`

	Title  string `gorm:"size:255;not null;default:'Attack Surface Chat'" json:"title"`
	Status string `gorm:"size:32;not null;default:'open';index" json:"status"`

	ContextJSON   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"context_json"`
	LastMessageAt *time.Time     `gorm:"index" json:"last_message_at"`
}

// AgentChatMessage stores user/assistant/tool messages for the target chat agent.
type AgentChatMessage struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	SessionID uint              `gorm:"not null;index" json:"session_id"`
	Session   *AgentChatSession `gorm:"foreignKey:SessionID" json:"-"`

	TargetID uint    `gorm:"not null;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	CreatedByUserID *uint `gorm:"index" json:"created_by_user_id"`
	CreatedByUser   *User `gorm:"foreignKey:CreatedByUserID" json:"-"`

	Role        string `gorm:"size:32;not null;index" json:"role"`
	MessageType string `gorm:"size:64;not null;default:'text';index" json:"message_type"`
	Content     string `gorm:"type:text" json:"content"`

	InputJSON  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"input_json"`
	OutputJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"output_json"`

	AgentRunID    *uint `gorm:"index" json:"agent_run_id"`
	AgentActionID *uint `gorm:"index" json:"agent_action_id"`
}
