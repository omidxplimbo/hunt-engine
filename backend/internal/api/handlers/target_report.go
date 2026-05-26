package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/auditlog"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
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

	_, ownerKey, _, _, ownerErr := currentAccountOwner(c)
	if ownerErr != nil {
		return ownerErr
	}
	if !featureflags.IsEnabledForOwner(ownerKey, featureflags.KeyTargetPDFReport) {
		return featureflags.DisabledResponse(c, featureflags.KeyTargetPDFReport)
	}

	pdfBytes, filename, err := targetpdf.Generate(target.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to generate target report",
			"error":   err.Error(),
		})
	}

	if uid, uidErr := currentUserID(c); uidErr == nil {
		entityID := target.ID
		_ = auditlog.Record(auditlog.Entry{
			ActorUserID: &uid,
			Action:      "target.report_pdf.download",
			EntityType:  "target",
			EntityID:    &entityID,
			TargetID:    &target.ID,
			IPAddress:   auditlog.ClientIP(c),
			UserAgent:   auditlog.UserAgent(c),
			Metadata: map[string]interface{}{
				"filename":    filename,
				"root_domain": target.RootDomain,
			},
		})
	}

	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filename))
	c.Set(fiber.HeaderCacheControl, "no-store")

	return c.Send(pdfBytes)
}
