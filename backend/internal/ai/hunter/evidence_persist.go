package hunter

import (
	"encoding/json"
	"log"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"gorm.io/gorm"
)

// EvidencePersister saves evidence to PostgreSQL so hunts survive restarts
// and reports can query real data.
type EvidencePersister struct {
	db       *gorm.DB
	targetID uint
	userID   uint
	ownerKey string
	agentID  string
}

// NewEvidencePersister creates a DB-backed evidence persister
func NewEvidencePersister(db *gorm.DB, targetID, userID uint, ownerKey, agentID string) *EvidencePersister {
	return &EvidencePersister{db: db, targetID: targetID, userID: userID, ownerKey: ownerKey, agentID: agentID}
}

// Save persists one evidence record. Never fails the hunt on DB errors —
// failures are logged only.
func (p *EvidencePersister) Save(e *Evidence) {
	data := map[string]any{}
	if e.Result != nil {
		data["http_result"] = e.Result
	}
	if e.Analysis != nil {
		data["analysis"] = e.Analysis
	}
	if e.Metadata != nil {
		data["metadata"] = e.Metadata
	}
	dataBytes, _ := json.Marshal(data)

	rec := models.HuntEvidence{
		UserID:     p.userID,
		OwnerKey:   p.ownerKey,
		TargetID:   p.targetID,
		AgentID:    p.agentID,
		TestType:   e.TestType,
		Target:     truncateStr(e.Target, 500),
		Parameter:  truncateStr(e.Parameter, 120),
		Payload:    truncateStr(e.Payload, 2000),
		Status:     e.Status,
		Confidence: e.Confidence,
		Severity:   e.Severity,
		PoC:        e.PoC,
		Notes:      e.Notes,
		Data:       dataBytes,
	}
	if err := p.db.Create(&rec).Error; err != nil {
		log.Printf("[Evidence] failed to persist evidence: %v", err)
	}
}

// SaveAll persists every evidence item in the store
func (p *EvidencePersister) SaveAll(store *EvidenceStore) {
	for _, e := range store.GetAll() {
		eCopy := e
		p.Save(&eCopy)
	}
}

// ListByTarget returns persisted evidence for a target
func ListByTarget(db *gorm.DB, targetID uint) ([]models.HuntEvidence, error) {
	var out []models.HuntEvidence
	err := db.Where("target_id = ?", targetID).
		Order("created_at DESC").Limit(500).Find(&out).Error
	return out, err
}
