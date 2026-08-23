package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	bugbounty "github.com/omidxplimbo/hunt-engine/backend/internal/reports/bugbounty"
)

// GenerateBugBountyReport generates a bug bounty report from hunt results
func GenerateBugBountyReport(c *fiber.Ctx) error {
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}

	// Load target
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Target not found"})
	}

	// Load hunt results from memory
	var memories []models.TargetMemoryItem
	database.DB.Where("target_id = ? AND memory_type = ?", targetID, "hunt_result").
		Order("created_at DESC").Limit(1).Find(&memories)

	if len(memories) == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "No hunt results found. Run a hunt first."})
	}

	// Run a quick hunt to get fresh evidence
	uid, err := currentUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	_, ownerKey, _, _, err := currentAccountOwner(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Create hunter agent and run a quick scan
	agent := hunter.NewHunterAgent(database.DB, target, nil, uid, ownerKey)
	result, err := agent.Hunt(c.Context(), "Generate report from existing evidence")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Generate report
	report := bugbounty.GenerateReport(result.Evidence, target.RootDomain)
	if report == nil {
		return c.Status(404).JSON(fiber.Map{"error": "No confirmed vulnerabilities found to report"})
	}

	// Return as markdown
	markdown := report.ToMarkdown()

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"report":   report,
			"markdown": markdown,
		},
	})
}

// DownloadBugBountyReport downloads the report as a markdown file
func DownloadBugBountyReport(c *fiber.Ctx) error {
	targetID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}

	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Target not found"})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	_, ownerKey, _, _, err := currentAccountOwner(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	agent := hunter.NewHunterAgent(database.DB, target, nil, uid, ownerKey)
	result, err := agent.Hunt(c.Context(), "Generate report")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	report := bugbounty.GenerateReport(result.Evidence, target.RootDomain)
	if report == nil {
		return c.Status(404).JSON(fiber.Map{"error": "No confirmed vulnerabilities found"})
	}

	markdown := report.ToMarkdown()
	filename := fmt.Sprintf("bug-bounty-report-%s-%s.md", target.RootDomain, time.Now().Format("2006-01-02"))

	c.Set("Content-Type", "text/markdown")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	return c.SendString(markdown)
}
