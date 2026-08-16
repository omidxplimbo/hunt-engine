package handlers

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/gorm"
)

// AuthContextRequest represents the input for creating/updating an auth context
type AuthContextRequest struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	IsActive      *bool                  `json:"is_active,omitempty"`
	AuthType      string                 `json:"auth_type"` // cookie, header, token, basic, oauth2
	Credentials   map[string]interface{} `json:"credentials"` // plaintext credentials (will be encrypted)
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ExpiresAt     *time.Time             `json:"expires_at,omitempty"`
}

// AuthContextResponse represents the output for an auth context (no sensitive data)
type AuthContextResponse struct {
	ID           uint                 `json:"id"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
	TargetID     uint                 `json:"target_id"`
	UserID       uint                 `json:"user_id"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	IsActive     bool                 `json:"is_active"`
	AuthType     string               `json:"auth_type"`
	Metadata     map[string]interface{} `json:"metadata"`
	ExpiresAt    *time.Time           `json:"expires_at,omitempty"`
	LastUsedAt   *time.Time           `json:"last_used_at,omitempty"`
	UsageCount   int                  `json:"usage_count"`
}

func currentUserID(c *fiber.Ctx) (uint, error) {
	val := c.Locals("user_id")
	switch v := val.(type) {
	case float64:
		if v <= 0 {
			return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
		}
		return uint(v), nil
	case int:
		if v <= 0 {
			return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
		}
		return uint(v), nil
	case uint:
		if v == 0 {
			return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
		}
		return v, nil
	case string:
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
		}
		return uint(n), nil
	default:
		return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
	}
}

func currentUserRole(c *fiber.Ctx) string {
	role, _ := c.Locals("role").(string)
	return strings.ToLower(strings.TrimSpace(role))
}

func isAdmin(c *fiber.Ctx) bool {
	return currentUserRole(c) == "admin"
}

func scopedTargetsDB(c *fiber.Ctx) (*gorm.DB, uint, error) {
	uid, err := currentUserID(c)
	if err != nil {
		return nil, 0, err
	}
	db := database.DB.Model(&models.Target{})
	if !isAdmin(c) {
		db = db.Where("created_by_user_id = ?", uid)
	}
	return db, uid, nil
}

func getAccessibleTarget(c *fiber.Ctx, targetID uint) (*models.Target, error) {
	db, _, err := scopedTargetsDB(c)
	if err != nil {
		return nil, err
	}
	var t models.Target
	if err := db.First(&t, targetID).Error; err != nil {
		// 404 هم برای not-found و هم برای not-authorized (برای جلوگیری از enumeration)
		return nil, fiber.NewError(fiber.StatusNotFound, "Target not found")
	}
	return &t, nil
}

// encryptCredentials simple base64 encoding (TODO: replace with proper AES encryption)
func encryptCredentials(creds map[string]interface{}) (string, error) {
	// TODO: Implement proper AES-256 encryption with a key from environment
	// For now, using base64 as placeholder
	data := creds // In real implementation, convert to JSON first
	jsonData := "" // json.Marshal(data)
	return base64.StdEncoding.EncodeToString([]byte(jsonData)), nil
}

// decryptCredentials simple base64 decoding (TODO: replace with proper AES decryption)
func decryptCredentials(encrypted string) (map[string]interface{}, error) {
	// TODO: Implement proper AES-256 decryption
	decoded, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}
	// In real implementation, unmarshal JSON here
	return map[string]interface{}{"data": string(decoded)}, nil
}

// CreateAuthContext creates a new auth context for a target
func CreateAuthContext(c *fiber.Ctx) error {
	targetIDParam := c.Params("targetID")
	targetID, err := strconv.ParseUint(targetIDParam, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid target ID")
	}

	target, err := getAccessibleTarget(c, uint(targetID))
	if err != nil {
		return err
	}

	var req AuthContextRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	// Validate required fields
	if req.Name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Name is required")
	}
	if req.AuthType == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Auth type is required")
	}
	if len(req.Credentials) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "Credentials are required")
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	// Encrypt credentials
	encryptedCreds, err := encryptCredentials(req.Credentials)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to encrypt credentials")
	}

	authCtx := models.AuthContext{
		TargetID:           uint(targetID),
		UserID:             uid,
		Name:               req.Name,
		Description:        req.Description,
		IsActive:           true,
		AuthType:           req.AuthType,
		EncryptedCredentials: encryptedCreds,
		Metadata:           "", // TODO: marshal metadata to JSON
		ExpiresAt:          req.ExpiresAt,
		CreatedByUserID:    &uid,
	}

	if req.IsActive != nil {
		authCtx.IsActive = *req.IsActive
	}

	if err := database.DB.Create(&authCtx).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to create auth context")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "success",
		"data":   authContextToResponse(authCtx),
	})
}

// ListAuthContexts lists all auth contexts for a target
func ListAuthContexts(c *fiber.Ctx) error {
	targetIDParam := c.Params("targetID")
	targetID, err := strconv.ParseUint(targetIDParam, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid target ID")
	}

	_, err = getAccessibleTarget(c, uint(targetID))
	if err != nil {
		return err
	}

	var contexts []models.AuthContext
	if err := database.DB.Where("target_id = ?", targetID).Order("created_at DESC").Find(&contexts).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to fetch auth contexts")
	}

	response := make([]AuthContextResponse, len(contexts))
	for i, ctx := range contexts {
		response[i] = authContextToResponse(ctx)
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   response,
	})
}

// GetAuthContext retrieves a single auth context by ID
func GetAuthContext(c *fiber.Ctx) error {
	targetIDParam := c.Params("targetID")
	contextIDParam := c.Params("contextID")
	
	targetID, err := strconv.ParseUint(targetIDParam, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid target ID")
	}
	
	contextID, err := strconv.ParseUint(contextIDParam, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid context ID")
	}

	_, err = getAccessibleTarget(c, uint(targetID))
	if err != nil {
		return err
	}

	var context models.AuthContext
	if err := database.DB.Where("id = ? AND target_id = ?", contextID, targetID).First(&context).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fiber.NewError(fiber.StatusNotFound, "Auth context not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to fetch auth context")
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   authContextToResponse(context),
	})
}

// UpdateAuthContext updates an existing auth context
func UpdateAuthContext(c *fiber.Ctx) error {
	targetIDParam := c.Params("targetID")
	contextIDParam := c.Params("contextID")
	
	targetID, err := strconv.ParseUint(targetIDParam, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid target ID")
	}
	
	contextID, err := strconv.ParseUint(contextIDParam, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid context ID")
	}

	_, err = getAccessibleTarget(c, uint(targetID))
	if err != nil {
		return err
	}

	var context models.AuthContext
	if err := database.DB.Where("id = ? AND target_id = ?", contextID, targetID).First(&context).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fiber.NewError(fiber.StatusNotFound, "Auth context not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to fetch auth context")
	}

	var req AuthContextRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	// Update fields
	if req.Name != "" {
		context.Name = req.Name
	}
	if req.Description != "" {
		context.Description = req.Description
	}
	if req.IsActive != nil {
		context.IsActive = *req.IsActive
	}
	if req.AuthType != "" {
		context.AuthType = req.AuthType
	}
	if len(req.Credentials) > 0 {
		encryptedCreds, err := encryptCredentials(req.Credentials)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to encrypt credentials")
		}
		context.EncryptedCredentials = encryptedCreds
	}
	if req.ExpiresAt != nil {
		context.ExpiresAt = req.ExpiresAt
	}

	if err := database.DB.Save(&context).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to update auth context")
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   authContextToResponse(context),
	})
}

// DeleteAuthContext deletes an auth context
func DeleteAuthContext(c *fiber.Ctx) error {
	targetIDParam := c.Params("targetID")
	contextIDParam := c.Params("contextID")
	
	targetID, err := strconv.ParseUint(targetIDParam, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid target ID")
	}
	
	contextID, err := strconv.ParseUint(contextIDParam, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid context ID")
	}

	_, err = getAccessibleTarget(c, uint(targetID))
	if err != nil {
		return err
	}

	result := database.DB.Where("id = ? AND target_id = ?", contextID, targetID).Delete(&models.AuthContext{})
	if result.Error != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to delete auth context")
	}
	if result.RowsAffected == 0 {
		return fiber.NewError(fiber.StatusNotFound, "Auth context not found")
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"message": "Auth context deleted successfully",
	})
}

// UseAuthContext marks an auth context as used and returns decrypted credentials
// This should only be called by authorized internal services
func UseAuthContext(c *fiber.Ctx) error {
	contextIDParam := c.Params("contextID")
	contextID, err := strconv.ParseUint(contextIDParam, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid context ID")
	}

	var context models.AuthContext
	if err := database.DB.First(&context, contextID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fiber.NewError(fiber.StatusNotFound, "Auth context not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to fetch auth context")
	}

	// Update usage stats
	now := time.Now()
	context.LastUsedAt = &now
	context.UsageCount++
	if err := database.DB.Save(&context).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to update usage stats")
	}

	// Decrypt credentials
	decryptedCreds, err := decryptCredentials(context.EncryptedCredentials)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to decrypt credentials")
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"auth_type":   context.AuthType,
			"credentials": decryptedCreds,
			"expires_at":  context.ExpiresAt,
		},
	})
}

func authContextToResponse(ctx models.AuthContext) AuthContextResponse {
	return AuthContextResponse{
		ID:         ctx.ID,
		CreatedAt:  ctx.CreatedAt,
		UpdatedAt:  ctx.UpdatedAt,
		TargetID:   ctx.TargetID,
		UserID:     ctx.UserID,
		Name:       ctx.Name,
		Description: ctx.Description,
		IsActive:   ctx.IsActive,
		AuthType:   ctx.AuthType,
		Metadata:   make(map[string]interface{}), // TODO: parse metadata JSON
		ExpiresAt:  ctx.ExpiresAt,
		LastUsedAt: ctx.LastUsedAt,
		UsageCount: ctx.UsageCount,
	}
}
