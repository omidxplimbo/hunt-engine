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
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/telegram"
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
			ID:                 user.ID,
			Username:           user.Username,
			Role:               user.Role,
			CreatedAt:          user.CreatedAt.Format(time.RFC3339),
			MaxConcurrentScans: user.MaxConcurrentScans,
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

// -----------------------------
// Telegram notification config
// -----------------------------

type telegramConfigResponse struct {
	Scope                       string   `json:"scope"`
	OwnerKey                    string   `json:"owner_key"`
	Enabled                     bool     `json:"enabled"`
	HasBotToken                 bool     `json:"has_bot_token"`
	ChatID                      string   `json:"chat_id"`
	EnabledEvents               []string `json:"enabled_events"`
	FreshAssetScreenshotEnabled bool     `json:"fresh_asset_screenshot_enabled"`
}

type telegramConfigRequest struct {
	Enabled                     *bool    `json:"enabled"`
	BotToken                    *string  `json:"bot_token"`
	ChatID                      *string  `json:"chat_id"`
	EnabledEvents               []string `json:"enabled_events"`
	FreshAssetScreenshotEnabled *bool    `json:"fresh_asset_screenshot_enabled"`
}

// GetMyTelegramConfig returns the current user's Telegram notification settings.
// Admin users share one admin-scoped Telegram config.
func GetMyTelegramConfig(c *fiber.Ctx) error {
	user, err := currentTelegramUser(c)
	if err != nil {
		return err
	}

	cfg, ownerKey, scope := getOrDefaultTelegramConfig(user)

	return c.JSON(fiber.Map{
		"status": "success",
		"data": telegramConfigResponse{
			Scope:                       scope,
			OwnerKey:                    ownerKey,
			Enabled:                     cfg.Enabled,
			HasBotToken:                 strings.TrimSpace(cfg.BotToken) != "",
			ChatID:                      cfg.ChatID,
			EnabledEvents:               parseTelegramEventList(cfg.EnabledEvents),
			FreshAssetScreenshotEnabled: cfg.FreshAssetScreenshotEnabled,
		},
	})
}

// PutMyTelegramConfig updates the current user's Telegram notification settings.
// Empty or omitted bot_token keeps the previous token.
func PutMyTelegramConfig(c *fiber.Ctx) error {
	user, err := currentTelegramUser(c)
	if err != nil {
		return err
	}

	req := new(telegramConfigRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	cfg, ownerKey, _ := getOrDefaultTelegramConfig(user)
	cfg.OwnerKey = ownerKey

	if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
		cfg.UserID = nil
	} else {
		cfg.UserID = &user.ID
	}

	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}

	if req.FreshAssetScreenshotEnabled != nil {
		cfg.FreshAssetScreenshotEnabled = *req.FreshAssetScreenshotEnabled
	}

	if req.ChatID != nil {
		cfg.ChatID = strings.TrimSpace(*req.ChatID)
	}

	if req.BotToken != nil {
		token := strings.TrimSpace(*req.BotToken)
		if token != "" {
			cfg.BotToken = token
		}
	}

	if req.EnabledEvents != nil {
		events, err := validateTelegramEvents(req.EnabledEvents)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		data, _ := json.Marshal(events)
		cfg.EnabledEvents = string(data)
	}

	if strings.TrimSpace(cfg.EnabledEvents) == "" {
		data, _ := json.Marshal(defaultTelegramEvents())
		cfg.EnabledEvents = string(data)
	}

	if err := database.DB.Save(&cfg).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save Telegram config"})
	}

	telegram.InvalidateConfigCache(cfg.OwnerKey, cfg.UserID)

	return c.JSON(fiber.Map{"status": "success", "message": "Telegram config saved"})
}

func currentTelegramUser(c *fiber.Ctx) (models.User, error) {
	uid, err := currentUserID(c)
	if err != nil {
		return models.User{}, err
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return models.User{}, c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return user, nil
}

func getOrDefaultTelegramConfig(user models.User) (models.UserTelegramConfig, string, string) {
	ownerKey, scope := telegramOwnerKeyForUser(user)

	var cfg models.UserTelegramConfig
	if err := database.DB.Where("owner_key = ?", ownerKey).First(&cfg).Error; err != nil {
		data, _ := json.Marshal(defaultTelegramEvents())
		cfg = models.UserTelegramConfig{
			OwnerKey:                    ownerKey,
			Enabled:                     false,
			EnabledEvents:               string(data),
			FreshAssetScreenshotEnabled: false,
		}

		if scope == "user" {
			cfg.UserID = &user.ID
		}
	}

	return cfg, ownerKey, scope
}

func telegramOwnerKeyForUser(user models.User) (string, string) {
	if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
		return "admin", "admin"
	}

	return fmt.Sprintf("user:%d", user.ID), "user"
}

func defaultTelegramEvents() []string {
	return []string{
		"fresh_asset",
		"asset_change_is_live",
		"asset_change_status_code",
		"asset_change_title",
		"asset_change_web_server",
		"asset_change_technologies",
		"asset_change_host_ip",
		"fresh_url",
	}
}

func validateTelegramEvents(input []string) ([]string, error) {
	allowed := map[string]struct{}{}
	for _, event := range defaultTelegramEvents() {
		allowed[event] = struct{}{}
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(input))

	for _, event := range input {
		event = strings.TrimSpace(event)
		if event == "" {
			continue
		}

		if _, ok := allowed[event]; !ok {
			return nil, fmt.Errorf("Unknown telegram notification event: %s", event)
		}

		if _, ok := seen[event]; ok {
			continue
		}

		seen[event] = struct{}{}
		out = append(out, event)
	}

	return out, nil
}

func parseTelegramEventList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultTelegramEvents()
	}

	var events []string
	if err := json.Unmarshal([]byte(raw), &events); err == nil {
		valid, err := validateTelegramEvents(events)
		if err == nil {
			return valid
		}
	}

	return defaultTelegramEvents()
}
