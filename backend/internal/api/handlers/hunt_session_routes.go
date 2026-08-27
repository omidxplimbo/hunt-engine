package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter"
)

// ListHuntSessions returns snapshots of all in-process hunt sessions for a
// target. Used by the UI's session list panel and by the operator who wants
// to resume/pause/cancel an active hunt (T6 covers the action endpoints).
func ListHuntSessions(c *fiber.Ctx) error {
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}

	if _, err := currentUserID(c); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	sessions := HuntSessions.ListForTarget(uint(targetID))
	return c.JSON(fiber.Map{
		"status": "success",
		"data":   sessions,
	})
}

// ensure unused-imports stay referenced if future T6 endpoints get folded in
var _ = hunter.SteerMessage
