package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils"
)

// GetMe اطلاعات کاربر فعلی را برمی‌گرداند
func GetMe(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": dto.MeResponse{
			ID:        user.ID,
			Username:  user.Username,
			Role:      user.Role,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		},
	})
}

// UpdateMe ویرایش پروفایل کاربر فعلی (فعلاً فقط username)
func UpdateMe(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req := new(dto.UpdateMeRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if req.Username != nil {
		u := strings.TrimSpace(*req.Username)
		if u == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Username cannot be empty"})
		}
		user.Username = u
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to update profile"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Profile updated"})
}

// ChangeMyPassword تغییر رمز عبور کاربر فعلی (با تایید رمز فعلی)
func ChangeMyPassword(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req := new(dto.ChangePasswordRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	req.CurrentPassword = strings.TrimSpace(req.CurrentPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "current_password and new_password are required"})
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if !utils.CheckPasswordHash(req.CurrentPassword, user.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Current password is incorrect"})
	}

	hash, _ := utils.HashPassword(req.NewPassword)
	user.Password = hash
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to change password"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Password changed"})
}

// DeleteMe حذف اکانت کاربر فعلی (با تایید رمز فعلی)
func DeleteMe(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req := new(dto.DeleteMeRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	req.CurrentPassword = strings.TrimSpace(req.CurrentPassword)
	if req.CurrentPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "current_password is required"})
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if !utils.CheckPasswordHash(req.CurrentPassword, user.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Current password is incorrect"})
	}

	// جلوگیری از حذف آخرین ادمین
	if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
		var otherAdmins int64
		database.DB.Model(&models.User{}).
			Where("role = ? AND id <> ?", "admin", user.ID).
			Count(&otherAdmins)
		if otherAdmins == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot delete the last admin user"})
		}
	}

	if err := database.DB.Delete(&models.User{}, uid).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete account"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Account deleted"})
}

// -----------------------------
// Subfinder provider config (per-user)
// -----------------------------

// GetMySubfinderProviders returns the current user's subfinder provider configs.
func GetMySubfinderProviders(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	var rows []models.SubfinderProviderConfig
	if err := database.DB.
		Where("user_id = ?", uid).
		Order("provider asc").
		Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load subfinder providers"})
	}

	items := make([]dto.SubfinderProviderItem, 0, len(rows))
	for _, r := range rows {
		var entries []interface{}
		_ = json.Unmarshal([]byte(r.Entries), &entries)
		items = append(items, dto.SubfinderProviderItem{
			Provider: r.Provider,
			Entries:  entries,
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"providers": items,
		},
	})
}

// PutMySubfinderProviders replaces the current user's provider list with the provided payload.
func PutMySubfinderProviders(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req := new(dto.PutMySubfinderProvidersRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	// Normalize + validate, build final rows
	type row struct {
		Provider string
		Entries  string
	}
	var final []row
	seen := make(map[string]bool)

	for _, p := range req.Providers {
		provider := strings.ToLower(strings.TrimSpace(p.Provider))
		if provider == "" {
			continue
		}
		if seen[provider] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Duplicate provider: %s", provider)})
		}
		seen[provider] = true

		if p.Entries == nil {
			p.Entries = []interface{}{}
		}
		entriesJSON, err := json.Marshal(p.Entries)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid entries for provider %s", provider)})
		}

		final = append(final, row{Provider: provider, Entries: string(entriesJSON)})
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start transaction"})
	}
	defer func() { _ = tx.Rollback() }()

	// Replace semantics: delete all then insert the new set
	// IMPORTANT: hard-delete (Unscoped) to avoid unique constraint conflicts with soft-deleted rows.
	// The table uses a unique index on (user_id, provider). Soft deletes keep rows, so recreating the
	// same provider would fail.
	if err := tx.Unscoped().Where("user_id = ?", uid).Delete(&models.SubfinderProviderConfig{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update subfinder providers"})
	}

	for _, r := range final {
		cfg := models.SubfinderProviderConfig{
			UserID:   uid,
			Provider: r.Provider,
			Entries:  r.Entries,
		}
		if err := tx.Create(&cfg).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update subfinder providers"})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update subfinder providers"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Subfinder providers updated"})
}

// DeleteMySubfinderProvider removes a single provider for the current user.
func DeleteMySubfinderProvider(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}
	provider := strings.ToLower(strings.TrimSpace(c.Params("provider")))
	if provider == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "provider is required"})
	}

	// IMPORTANT: hard-delete (Unscoped) to allow re-adding the same provider later without unique conflicts.
	if err := database.DB.Unscoped().
		Where("user_id = ? AND provider = ?", uid, provider).
		Delete(&models.SubfinderProviderConfig{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete provider"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Provider deleted"})
}



