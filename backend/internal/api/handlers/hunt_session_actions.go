package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter"
)

// PauseHuntSession enqueues a pause command on the session's SteerCh.
// The AgentLoop's drainSteer picks it up between turns and blocks until
// resume/cancel/message arrives.
func PauseHuntSession(c *fiber.Ctx) error {
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}
	sessionID := c.Params("sid")
	if sessionID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing session id"})
	}
	if !sessionOwnedBy(c, uint(targetID), sessionID) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden"})
	}
	sess := HuntSessions.Get(uint(targetID), sessionID)
	if sess == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Session not found"})
	}
	sess.EnqueueSteer(hunter.SteerCommand{Type: hunter.SteerPause})
	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "pause requested",
		"data":    sess.Snapshot(),
	})
}

// ResumeHuntSession clears the paused flag.
func ResumeHuntSession(c *fiber.Ctx) error {
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}
	sessionID := c.Params("sid")
	if sessionID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing session id"})
	}
	if !sessionOwnedBy(c, uint(targetID), sessionID) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden"})
	}
	sess := HuntSessions.Get(uint(targetID), sessionID)
	if sess == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Session not found"})
	}
	sess.EnqueueSteer(hunter.SteerCommand{Type: hunter.SteerResume})
	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "resume requested",
		"data":    sess.Snapshot(),
	})
}

// CancelHuntSession calls the session's CancelFn. The agent loop's
// context is cancelled; Run returns with ctx.Err() on the next check.
func CancelHuntSession(c *fiber.Ctx) error {
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}
	sessionID := c.Params("sid")
	if sessionID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing session id"})
	}
	if !sessionOwnedBy(c, uint(targetID), sessionID) {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden"})
	}
	sess := HuntSessions.Get(uint(targetID), sessionID)
	if sess == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Session not found"})
	}
	sess.EnqueueSteer(hunter.SteerCommand{Type: hunter.SteerCancel})
	if sess.CancelFn != nil {
		sess.CancelFn()
	}
	sess.MarkStatus("cancelled")
	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "cancel requested",
		"data":    sess.Snapshot(),
	})
}

// sessionOwnedBy returns true when the user_id on the caller matches
// the session's user_id. Admin users can act on any session.
func sessionOwnedBy(c *fiber.Ctx, targetID uint, sessionID string) bool {
	uid, err := currentUserID(c)
	if err != nil {
		return false
	}
	sess := HuntSessions.Get(targetID, sessionID)
	if sess == nil {
		return false
	}
	role, _ := c.Locals("role").(string)
	if role == "admin" {
		return true
	}
	return sess.UserID == uid
}
