package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/reports/targetpdf"
)

// DownloadTargetPDFReport returns a professional PDF report for one accessible target.
func DownloadTargetPDFReport(c *fiber.Ctx) error {
	id := c.Params("id")

	var targetID uint
	_, _ = fmt.Sscanf(id, "%d", &targetID)
	if targetID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid target id",
		})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	pdfBytes, filename, err := targetpdf.Generate(target.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to generate target report",
			"error":   err.Error(),
		})
	}

	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filename))
	c.Set(fiber.HeaderCacheControl, "no-store")

	return c.Send(pdfBytes)
}
