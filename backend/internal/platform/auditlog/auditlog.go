package auditlog

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/datatypes"
)

type Entry struct {
	ActorUserID *uint
	ActorName   string
	Action      string
	EntityType  string
	EntityID    *uint
	TargetID    *uint
	IPAddress   string
	UserAgent   string
	Metadata    interface{}
}

// Record writes an audit log entry. Callers should treat audit logging as best-effort;
// user-facing actions should not fail just because the audit write failed.
func Record(entry Entry) error {
	if entry.Action == "" {
		return fmt.Errorf("audit action is required")
	}
	if entry.EntityType == "" {
		return fmt.Errorf("audit entity type is required")
	}

	raw := datatypes.JSON([]byte("{}"))
	if entry.Metadata != nil {
		b, err := json.Marshal(entry.Metadata)
		if err != nil {
			return fmt.Errorf("marshal audit metadata: %w", err)
		}
		raw = datatypes.JSON(b)
	}

	row := models.AuditLog{
		ActorUserID: entry.ActorUserID,
		ActorName:   entry.ActorName,
		Action:      entry.Action,
		EntityType:  entry.EntityType,
		EntityID:    entry.EntityID,
		TargetID:    entry.TargetID,
		IPAddress:   entry.IPAddress,
		UserAgent:   entry.UserAgent,
		Metadata:    raw,
	}

	return database.DB.Create(&row).Error
}

func ClientIP(c *fiber.Ctx) string {
	if c == nil {
		return ""
	}
	return c.IP()
}

func UserAgent(c *fiber.Ctx) string {
	if c == nil {
		return ""
	}
	return c.Get(fiber.HeaderUserAgent)
}
