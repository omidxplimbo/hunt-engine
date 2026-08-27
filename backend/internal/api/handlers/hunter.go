package handlers

import (
	"context"
	"log"
	"strconv"
	"time"

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

// StartHunt creates a HuntSession and spawns the agent in a background
// goroutine. Returns immediately with the session_id so the client can
// attach the WS handler for live progress + steering (T5).
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

	mode := req.Mode
	if mode != "multi" {
		mode = "single"
	}

	// Resolve LLM config
	llmCfg, _ := llmclient.ResolveDefault(ownerKey)

	// Create session
	sess := hunter.NewHuntSession(uint(targetID), uid, ownerKey, mode, req.Objective)

	// Detached context — not tied to the HTTP request so the hunt survives
	// the client disconnecting, and so the WS handler can cancel it via
	// session.CancelFn (T6).
	huntCtx, cancel := context.WithCancel(context.Background())
	sess.CancelFn = cancel

	// Build the agent in the request goroutine (so a misconfigured target
	// fails the request instead of starting an orphaned goroutine) then
	// dispatch the long-running work.
	agent := hunter.NewHunterAgent(database.DB, target, llmCfg, uid, ownerKey)
	targetIDUint := uint(targetID)

	HuntSessions.Add(sess)

	go func() {
		defer cancel()
		defer HuntSessions.Remove(targetIDUint, sess.ID)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Hunter] session %s panicked: %v", sess.ID, r)
				sess.SetError("agent panic")
				sess.MarkStatus("failed")
			}
		}()

		var result *hunter.HunterResult
		var runErr error

		progressFn := func(tid uint, ev hunter.AgentEvent) {
			PublishHuntEvent(tid, ev)
		}

		opts := []hunter.HuntOption{
			hunter.WithProgress(targetIDUint, progressFn),
			hunter.WithPersistence(database.DB, targetIDUint, uid, ownerKey),
			hunter.WithSession(sess),
		}

		if mode == "multi" {
			result, runErr = agent.HuntMultiAgent(huntCtx, req.Objective, opts...)
		} else {
			result, runErr = agent.Hunt(huntCtx, req.Objective, opts...)
		}

		switch {
		case runErr != nil:
			sess.SetError(runErr.Error())
			sess.MarkStatus("failed")
			PublishHuntEvent(targetIDUint, hunter.AgentEvent{
				Type:      "error",
				Detail:    runErr.Error(),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			})
		case result != nil:
			sess.SetResult(result.Summary, result.VulnsFound)
			sess.MarkStatus("completed")
		default:
			sess.MarkStatus("completed")
		}

		PublishHuntEvent(targetIDUint, hunter.AgentEvent{
			Type:      "session_done",
			Detail:    sess.Snapshot().Status,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}()

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"session_id": sess.ID,
			"status":     sess.Status,
			"ws_path":    "/api/targets/" + strconv.Itoa(targetID) + "/hunter/ws",
		},
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
