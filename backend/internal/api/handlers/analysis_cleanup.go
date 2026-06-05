package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/auditlog"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/gorm"
)

func parseUintRouteParam(c *fiber.Ctx, name string) (uint, error) {
	raw := strings.TrimSpace(c.Params(name))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return uint(id), nil
}

func recordTargetDeleteAudit(c *fiber.Ctx, uid uint, target *models.Target, action string, entityType string, entityID uint, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["target_id"] = target.ID
	metadata["root_domain"] = target.RootDomain
	metadata["deleted"] = true

	id := entityID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      action,
		EntityType:  entityType,
		EntityID:    &id,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata:    metadata,
	})
}

// DeleteTargetAIAnalysis soft-deletes one AI analysis row for an accessible target.
func DeleteTargetAIAnalysis(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	rowID, err := parseUintRouteParam(c, "analysis_id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var row models.AIAnalysis
	if err := database.DB.Where("id = ? AND target_id = ?", rowID, target.ID).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "AI analysis not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	if err := database.DB.Delete(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	recordTargetDeleteAudit(c, uid, target, "target.ai_analysis.delete", "ai_analysis", row.ID, map[string]interface{}{
		"provider": row.Provider,
		"model":    row.Model,
		"source":   row.Source,
	})

	return c.JSON(fiber.Map{"status": "success", "message": "AI analysis deleted"})
}

// DeleteTargetAIRecommendation soft-deletes one AI recommendation row for an accessible target.
func DeleteTargetAIRecommendation(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	rowID, err := parseUintRouteParam(c, "recommendation_id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var row models.AIRecommendation
	if err := database.DB.Where("id = ? AND target_id = ?", rowID, target.ID).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "AI recommendation not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	if err := database.DB.Delete(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	recordTargetDeleteAudit(c, uid, target, "target.ai_recommendation.delete", "ai_recommendation", row.ID, map[string]interface{}{
		"recommendation_type": row.RecommendationType,
		"priority":            row.Priority,
		"status":              row.Status,
	})

	return c.JSON(fiber.Map{"status": "success", "message": "AI recommendation deleted"})
}

// DeleteTargetAgentRun soft-deletes one advisory agent run for an accessible target.
func DeleteTargetAgentRun(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	runID, err := parseUintRouteParam(c, "run_id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var row models.AgentRun
	if err := database.DB.Where("id = ? AND target_id = ?", runID, target.ID).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "agent run not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	if err := database.DB.Delete(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	recordTargetDeleteAudit(c, uid, target, "target.agent_run.delete", "agent_run", row.ID, map[string]interface{}{
		"agent_type":    row.AgentType,
		"provider":      row.Provider,
		"model":         row.Model,
		"status":        row.Status,
		"policy_status": row.PolicyStatus,
	})

	return c.JSON(fiber.Map{"status": "success", "message": "agent run deleted"})
}

// DeleteTargetAgentAction soft-deletes one agent action for an accessible target.
func DeleteTargetAgentAction(c *fiber.Ctx) error {
	if err := ensureAgentActionsEnabled(c); err != nil {
		return err
	}

	target, row, err := loadAccessibleAgentAction(c)
	if err != nil {
		if fe, ok := err.(*fiber.Error); ok {
			return c.Status(fe.Code).JSON(fiber.Map{"status": "error", "message": fe.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	if row.Status == models.AgentActionStatusExecuted {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  "error",
			"message": "executed actions cannot be deleted from the UI; keep the audit trail intact",
		})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	if err := database.DB.Delete(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	recordTargetDeleteAudit(c, uid, &target, "target.agent_action.delete", "agent_action", row.ID, map[string]interface{}{
		"action_type":   row.ActionType,
		"status":        row.Status,
		"policy_status": row.PolicyStatus,
		"risk_level":    row.RiskLevel,
	})

	return c.JSON(fiber.Map{"status": "success", "message": "agent action deleted"})
}

// DeleteTargetAgentChatSession soft-deletes one chat session and its messages.
func DeleteTargetAgentChatSession(c *fiber.Ctx) error {
	if err := ensureAgentChatEnabled(c); err != nil {
		return err
	}

	target, session, err := loadAccessibleAgentChatSession(c)
	if err != nil {
		if fe, ok := err.(*fiber.Error); ok {
			return c.Status(fe.Code).JSON(fiber.Map{"status": "error", "message": fe.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	var messageCount int64
	_ = database.DB.Model(&models.AgentChatMessage{}).
		Where("session_id = ? AND target_id = ?", session.ID, target.ID).
		Count(&messageCount).Error

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ? AND target_id = ?", session.ID, target.ID).
			Delete(&models.AgentChatMessage{}).Error; err != nil {
			return err
		}
		return tx.Delete(session).Error
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	recordTargetDeleteAudit(c, uid, target, "target.agent_chat.session.delete", "agent_chat_session", session.ID, map[string]interface{}{
		"title":         session.Title,
		"status":        session.Status,
		"message_count": messageCount,
	})

	return c.JSON(fiber.Map{"status": "success", "message": "agent chat session deleted"})
}
