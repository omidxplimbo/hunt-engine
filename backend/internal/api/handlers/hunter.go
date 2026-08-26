package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/llmclient"
)

type startHuntRequest struct {
	Objective string `json:"objective"`
	// Mode: "single" (one agent, default) or "multi" (supervisor + workers per bug class)
	Mode string `json:"mode,omitempty"`
}

// StartHunt starts a new hunting session on a target
func StartHunt(c *fiber.Ctx) error {
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	_, ownerKey, _, _, err := currentAccountOwner(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Load target
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Target not found"})
	}

	// Parse request
	req := new(startHuntRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Objective == "" {
		req.Objective = "Find vulnerabilities on this target"
	}

	// Resolve LLM config
	llmCfg, _ := llmclient.ResolveDefault(ownerKey)

	// Create and run hunter agent
	agent := hunter.NewHunterAgent(database.DB, target, llmCfg, uid, ownerKey)
	targetIDUint := uint(targetID)

	var result *hunter.HunterResult
	if req.Mode == "multi" {
		multi, err := agent.HuntMultiAgent(c.Context(), req.Objective,
			hunter.WithProgress(targetIDUint, handlersPublishEvent),
			hunter.WithPersistence(database.DB, targetIDUint, uid, ownerKey))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		result = multi
	} else {
		result, err = agent.Hunt(c.Context(), req.Objective,
			hunter.WithProgress(targetIDUint, handlersPublishEvent),
			hunter.WithPersistence(database.DB, targetIDUint, uid, ownerKey))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   result,
	})
}

// handlersPublishEvent adapts hunter.AgentEvent to the WebSocket hub
func handlersPublishEvent(targetID uint, ev hunter.AgentEvent) {
	PublishHuntEvent(targetID, ev)
}

// GetHuntResults returns all hunt results for a target
func GetHuntResults(c *fiber.Ctx) error {
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}

	// Load hunt results from memory
	var memories []models.TargetMemoryItem
	database.DB.Where("target_id = ? AND memory_type = ?", targetID, "hunt_result").
		Order("created_at DESC").Find(&memories)

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   memories,
	})
}

// GetHuntEvidence returns persisted hunt evidence for a target
func GetHuntEvidence(c *fiber.Ctx) error {
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}

	evidence, err := hunter.ListByTarget(database.DB, uint(targetID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   evidence,
	})
}
