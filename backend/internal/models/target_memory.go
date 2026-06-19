package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	TargetMemorySourceTarget               = "target"
	TargetMemorySourcePolicy               = "policy"
	TargetMemorySourceAsset                = "asset"
	TargetMemorySourceURL                  = "url"
	TargetMemorySourceFinding              = "finding"
	TargetMemorySourceEvidence             = "evidence"
	TargetMemorySourceBugTestRun           = "bug_test_run"
	TargetMemorySourceBugTestResult        = "bug_test_result"
	TargetMemorySourceControlledTestRun    = "controlled_test_run"
	TargetMemorySourceControlledTestResult = "controlled_test_result"
	TargetMemorySourceAgentAction          = "agent_action"
	TargetMemorySourceChatMessage          = "chat_message"
	TargetMemorySourceUserNote             = "user_note"

	TargetMemoryTypeOverview                = "target_overview"
	TargetMemoryTypeAttackSurface           = "attack_surface"
	TargetMemoryTypeEndpointNote            = "endpoint_note"
	TargetMemoryTypeParameterNote           = "parameter_note"
	TargetMemoryTypeTechnologyNote          = "technology_note"
	TargetMemoryTypeVulnerabilityHypothesis = "vulnerability_hypothesis"
	TargetMemoryTypeTestResult              = "test_result"
	TargetMemoryTypeFailedTest              = "failed_test"
	TargetMemoryTypeSuccessfulTest          = "successful_test"
	TargetMemoryTypeFindingEvidence         = "finding_evidence"
	TargetMemoryTypePolicyConstraint        = "policy_constraint"
	TargetMemoryTypeUserDecision            = "user_decision"
	TargetMemoryTypeOperatorSummary         = "operator_summary"

	TargetMemoryEventCreated = "memory.created"
	TargetMemoryEventUpdated = "memory.updated"
	TargetMemoryEventChunked = "memory.chunked"
	TargetMemoryEventUsed    = "memory.used"
)

type TargetMemoryItem struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey string `gorm:"size:80;index" json:"owner_key"`
	TargetID uint   `gorm:"not null;index" json:"target_id"`

	SourceType string `gorm:"size:96;not null;index" json:"source_type"`
	SourceID   *uint  `gorm:"index" json:"source_id"`

	MemoryType string `gorm:"size:96;not null;index" json:"memory_type"`
	Title      string `gorm:"size:255;not null" json:"title"`
	Content    string `gorm:"type:text;not null" json:"content"`
	Summary    string `gorm:"type:text" json:"summary"`

	Tags       datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"tags"`
	Importance int            `gorm:"not null;default:50;index" json:"importance"`
	Confidence int            `gorm:"not null;default:50;index" json:"confidence"`

	SourceHash string         `gorm:"size:128;index" json:"source_hash"`
	Metadata   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
}

type TargetMemoryChunk struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID       uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey     string `gorm:"size:80;index" json:"owner_key"`
	TargetID     uint   `gorm:"not null;index" json:"target_id"`
	MemoryItemID uint   `gorm:"not null;index" json:"memory_item_id"`

	ChunkIndex int    `gorm:"not null;default:0;index" json:"chunk_index"`
	ChunkText  string `gorm:"type:text;not null" json:"chunk_text"`
	TokenCount int    `gorm:"not null;default:0" json:"token_count"`

	EmbeddingProvider string         `gorm:"size:96;index" json:"embedding_provider"`
	EmbeddingModel    string         `gorm:"size:128;index" json:"embedding_model"`
	Embedding         datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"embedding"`

	SourceHash string         `gorm:"size:128;index" json:"source_hash"`
	Metadata   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	MemoryItem *TargetMemoryItem `gorm:"foreignKey:MemoryItemID" json:"-"`
}

type TargetMemoryEvent struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey string `gorm:"size:80;index" json:"owner_key"`
	TargetID uint   `gorm:"not null;index" json:"target_id"`

	EventType  string `gorm:"size:128;not null;index" json:"event_type"`
	SourceType string `gorm:"size:96;index" json:"source_type"`
	SourceID   *uint  `gorm:"index" json:"source_id"`

	MemoryItemID  *uint `gorm:"index" json:"memory_item_id"`
	MemoryChunkID *uint `gorm:"index" json:"memory_chunk_id"`

	BeforeJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"before_json"`
	AfterJSON  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"after_json"`
	Metadata   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
}
