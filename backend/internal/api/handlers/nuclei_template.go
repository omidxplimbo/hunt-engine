package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/gorm"
)

const (
	defaultNucleiCustomTemplatesDir = "/data/nuclei/custom"
	maxNucleiTemplateBytes          = 1024 * 1024
	nucleiValidationTimeout         = 30 * time.Second
	nucleiTemplateAPICacheTTL       = 30 * time.Second
)

var nucleiTemplateNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}\.ya?ml$`)

var allowedNucleiTemplatePlacements = map[string]struct{}{
	"root":       {},
	"shared":     {},
	"safe":       {},
	"fast":       {},
	"exposure":   {},
	"balanced":   {},
	"misconfig":  {},
	"cves":       {},
	"cves-light": {},
	"full":       {},
	"custom":     {},
}

type nucleiTemplateCacheEntry struct {
	expiresAt time.Time
	templates []dto.NucleiTemplateResponse
}

var nucleiTemplateAPICache = struct {
	sync.RWMutex
	byUser map[uint]nucleiTemplateCacheEntry
}{byUser: map[uint]nucleiTemplateCacheEntry{}}

func nucleiCustomTemplatesDir() string {
	dir := strings.TrimSpace(os.Getenv("NUCLEI_CUSTOM_TEMPLATES_DIR"))
	if dir == "" {
		dir = defaultNucleiCustomTemplatesDir
	}
	return filepath.Clean(dir)
}

func normalizeNucleiTemplatePlacement(placement string) (string, error) {
	placement = strings.ToLower(strings.TrimSpace(placement))
	if placement == "" {
		placement = "root"
	}
	if _, ok := allowedNucleiTemplatePlacements[placement]; !ok {
		return "", fmt.Errorf("invalid template placement %q", placement)
	}
	return placement, nil
}

func validateNucleiTemplateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("template name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) || strings.Contains(name, "..") {
		return "", errors.New("template name must be a simple .yaml or .yml filename")
	}
	if !nucleiTemplateNamePattern.MatchString(name) {
		return "", errors.New("template name must match ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}\\.ya?ml$")
	}
	return name, nil
}

func nucleiTemplateChecksum(content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func nucleiTemplateRelativePath(placement, name string) string {
	if placement == "root" {
		return name
	}
	return filepath.ToSlash(filepath.Join(placement, name))
}

func nucleiTemplateRuntimePath(userID uint, placement, name string) (string, error) {
	placement, err := normalizeNucleiTemplatePlacement(placement)
	if err != nil {
		return "", err
	}
	name, err = validateNucleiTemplateName(name)
	if err != nil {
		return "", err
	}

	root := filepath.Join(nucleiCustomTemplatesDir(), "users", fmt.Sprintf("%d", userID))
	path := filepath.Join(root, name)
	if placement != "root" {
		path = filepath.Join(root, placement, name)
	}

	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("resolved template path escapes custom template directory")
	}
	return path, nil
}

func templateResponseFromModel(t models.NucleiTemplate, includeContent bool) dto.NucleiTemplateResponse {
	path, _ := nucleiTemplateRuntimePath(t.UserID, t.Placement, t.Name)
	content := ""
	if includeContent {
		content = t.Content
	}
	return dto.NucleiTemplateResponse{
		ID:               t.ID,
		UserID:           t.UserID,
		Name:             t.Name,
		Placement:        t.Placement,
		RelativePath:     nucleiTemplateRelativePath(t.Placement, t.Name),
		Path:             path,
		SizeBytes:        int64(len([]byte(t.Content))),
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
		Content:          content,
		Enabled:          t.Enabled,
		Source:           t.Source,
		Checksum:         t.Checksum,
		ValidationStatus: t.ValidationStatus,
		ValidationError:  t.ValidationError,
		LastValidatedAt:  t.LastValidatedAt,
		IsAIGenerated:    t.IsAIGenerated,
		RequiresApproval: t.RequiresApproval,
	}
}

func cachedNucleiTemplates(userID uint) ([]dto.NucleiTemplateResponse, bool) {
	now := time.Now()
	nucleiTemplateAPICache.RLock()
	entry, ok := nucleiTemplateAPICache.byUser[userID]
	nucleiTemplateAPICache.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		return nil, false
	}
	out := make([]dto.NucleiTemplateResponse, len(entry.templates))
	copy(out, entry.templates)
	return out, true
}

func setCachedNucleiTemplates(userID uint, templates []dto.NucleiTemplateResponse) {
	out := make([]dto.NucleiTemplateResponse, len(templates))
	copy(out, templates)
	nucleiTemplateAPICache.Lock()
	nucleiTemplateAPICache.byUser[userID] = nucleiTemplateCacheEntry{expiresAt: time.Now().Add(nucleiTemplateAPICacheTTL), templates: out}
	nucleiTemplateAPICache.Unlock()
}

func invalidateNucleiTemplateCache(userID uint) {
	nucleiTemplateAPICache.Lock()
	delete(nucleiTemplateAPICache.byUser, userID)
	nucleiTemplateAPICache.Unlock()
}

func writeNucleiTemplateRuntimeCache(t models.NucleiTemplate) error {
	if !t.Enabled {
		return removeNucleiTemplateRuntimeCache(t.UserID, t.Placement, t.Name)
	}
	path, err := nucleiTemplateRuntimePath(t.UserID, t.Placement, t.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	desired := strings.TrimSpace(t.Content) + "\n"
	if existing, err := os.ReadFile(path); err == nil && string(existing) == desired {
		return nil
	}
	return os.WriteFile(path, []byte(desired), 0o600)
}

func removeNucleiTemplateRuntimeCache(userID uint, placement, name string) error {
	path, err := nucleiTemplateRuntimePath(userID, placement, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func queryNucleiTemplateForUser(userID uint, name, placement string) (models.NucleiTemplate, error) {
	name, err := validateNucleiTemplateName(name)
	if err != nil {
		return models.NucleiTemplate{}, err
	}

	q := database.DB.Where("user_id = ? AND name = ?", userID, name)
	if strings.TrimSpace(placement) != "" {
		cleanPlacement, err := normalizeNucleiTemplatePlacement(placement)
		if err != nil {
			return models.NucleiTemplate{}, err
		}
		q = q.Where("placement = ?", cleanPlacement)
	}

	var templates []models.NucleiTemplate
	if err := q.Order("placement ASC, id ASC").Find(&templates).Error; err != nil {
		return models.NucleiTemplate{}, err
	}
	if len(templates) == 0 {
		return models.NucleiTemplate{}, gorm.ErrRecordNotFound
	}
	if len(templates) > 1 {
		return models.NucleiTemplate{}, fmt.Errorf("multiple templates named %q exist; provide placement", name)
	}
	return templates[0], nil
}

// ListNucleiTemplates returns the authenticated user's custom Nuclei templates from Postgres.
func ListNucleiTemplates(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	if templates, ok := cachedNucleiTemplates(userID); ok {
		return c.JSON(fiber.Map{"status": "success", "data": templates, "count": len(templates), "cached": true})
	}

	var records []models.NucleiTemplate
	if err := database.DB.Where("user_id = ?", userID).Order("placement ASC, name ASC").Find(&records).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to read templates", "error": err.Error()})
	}

	templates := make([]dto.NucleiTemplateResponse, 0, len(records))
	for _, record := range records {
		templates = append(templates, templateResponseFromModel(record, false))
	}
	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Placement == templates[j].Placement {
			return templates[i].Name < templates[j].Name
		}
		return templates[i].RelativePath < templates[j].RelativePath
	})
	setCachedNucleiTemplates(userID, templates)

	return c.JSON(fiber.Map{"status": "success", "data": templates, "count": len(templates), "cached": false})
}

// GetNucleiTemplate returns one custom template including its YAML content.
func GetNucleiTemplate(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	template, err := queryNucleiTemplateForUser(userID, c.Params("name"), c.Query("placement"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Template not found"})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "data": templateResponseFromModel(template, true)})
}

// UpsertNucleiTemplate validates and stores a custom Nuclei template.
func UpsertNucleiTemplate(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	req := new(dto.UpsertNucleiTemplateRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body", "error": err.Error()})
	}

	placement, err := normalizeNucleiTemplatePlacement(req.Placement)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}
	name, err := validateNucleiTemplateName(req.Name)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Template content is required"})
	}
	if len([]byte(content)) > maxNucleiTemplateBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"status": "error", "message": "Template content exceeds 1 MiB limit"})
	}

	validationStatus := "unknown"
	validationError := ""
	var lastValidatedAt *time.Time
	shouldValidate := true
	if req.Validate != nil {
		shouldValidate = *req.Validate
	}
	if shouldValidate {
		validation := runNucleiTemplateValidation(name, content)
		now := time.Now().UTC()
		lastValidatedAt = &now
		if !validation.Valid {
			validationStatus = "invalid"
			validationError = validation.Error
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Template validation failed", "validation": validation})
		}
		validationStatus = "valid"
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	checksum := nucleiTemplateChecksum(content)
	var record models.NucleiTemplate
	err = database.DB.Where("user_id = ? AND placement = ? AND name = ?", userID, placement, name).First(&record).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to load template", "error": err.Error()})
	}

	if record.ID == 0 {
		record = models.NucleiTemplate{UserID: userID, Name: name, Placement: placement, Source: "manual"}
	}
	record.Content = content
	record.Enabled = enabled
	record.Checksum = checksum
	record.ValidationStatus = validationStatus
	record.ValidationError = validationError
	record.LastValidatedAt = lastValidatedAt
	if record.Source == "" {
		record.Source = "manual"
	}

	if err := database.DB.Save(&record).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to save template", "error": err.Error()})
	}
	if err := writeNucleiTemplateRuntimeCache(record); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Template saved but runtime cache could not be updated", "error": err.Error()})
	}
	invalidateNucleiTemplateCache(userID)

	return c.JSON(fiber.Map{"status": "success", "message": "Template saved", "data": templateResponseFromModel(record, true)})
}

// ValidateNucleiTemplate validates posted template content or an already saved custom template.
func ValidateNucleiTemplate(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	req := new(dto.ValidateNucleiTemplateRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body", "error": err.Error()})
	}

	if _, err := normalizeNucleiTemplatePlacement(req.Placement); err != nil && strings.TrimSpace(req.Placement) != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	name := strings.TrimSpace(req.Name)
	content := strings.TrimSpace(req.Content)
	if content == "" {
		if name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Provide either content or an existing template name"})
		}
		template, err := queryNucleiTemplateForUser(userID, name, req.Placement)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Template not found"})
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
		}
		content = template.Content
	}
	if name != "" {
		if _, err := validateNucleiTemplateName(name); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
		}
	}
	if len([]byte(content)) > maxNucleiTemplateBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"status": "error", "message": "Template content exceeds 1 MiB limit"})
	}

	validation := runNucleiTemplateValidation(name, content)
	status := fiber.StatusOK
	if !validation.Valid {
		status = fiber.StatusBadRequest
	}
	return c.Status(status).JSON(fiber.Map{"status": "success", "data": validation})
}

// DeleteNucleiTemplate removes one custom Nuclei template.
func DeleteNucleiTemplate(c *fiber.Ctx) error {
	userID, err := currentUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	template, err := queryNucleiTemplateForUser(userID, c.Params("name"), c.Query("placement"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Template not found"})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	if err := database.DB.Delete(&template).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete template", "error": err.Error()})
	}
	if err := removeNucleiTemplateRuntimeCache(template.UserID, template.Placement, template.Name); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Template deleted but runtime cache could not be removed", "error": err.Error()})
	}
	invalidateNucleiTemplateCache(userID)

	return c.JSON(fiber.Map{"status": "success", "message": "Template deleted"})
}

func runNucleiTemplateValidation(name, content string) dto.NucleiTemplateValidationResponse {
	resp := dto.NucleiTemplateValidationResponse{Name: strings.TrimSpace(name)}
	if strings.TrimSpace(content) == "" {
		resp.Valid = false
		resp.Error = "template content is empty"
		return resp
	}
	if _, err := exec.LookPath("nuclei"); err != nil {
		resp.Valid = false
		resp.Error = "nuclei binary not found in PATH"
		return resp
	}

	tmpDir, err := os.MkdirTemp("", "hunt-nuclei-template-validate-*")
	if err != nil {
		resp.Valid = false
		resp.Error = fmt.Sprintf("failed to create temp directory: %v", err)
		return resp
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "template.yaml")
	if err := os.WriteFile(tmpPath, []byte(content), 0o600); err != nil {
		resp.Valid = false
		resp.Error = fmt.Sprintf("failed to write temp template: %v", err)
		return resp
	}

	ctx, cancel := context.WithTimeout(context.Background(), nucleiValidationTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nuclei", "-validate", "-t", tmpPath, "-silent")
	output, err := cmd.CombinedOutput()
	resp.Output = strings.TrimSpace(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		resp.Valid = false
		resp.Error = "template validation timed out"
		return resp
	}
	if err != nil {
		resp.Valid = false
		if resp.Output != "" {
			resp.Error = resp.Output
		} else {
			resp.Error = err.Error()
		}
		return resp
	}
	resp.Valid = true
	return resp
}
