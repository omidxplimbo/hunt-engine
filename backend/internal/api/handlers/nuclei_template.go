package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
)

const (
	defaultNucleiCustomTemplatesDir = "/data/nuclei/custom"
	maxNucleiTemplateBytes          = 1024 * 1024
	nucleiValidationTimeout         = 30 * time.Second
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

func nucleiTemplatePath(placement, name string) (string, string, error) {
	cleanPlacement, err := normalizeNucleiTemplatePlacement(placement)
	if err != nil {
		return "", "", err
	}

	cleanName, err := validateNucleiTemplateName(name)
	if err != nil {
		return "", "", err
	}

	dir := nucleiCustomTemplatesDir()
	templateDir := dir
	relativePath := cleanName
	if cleanPlacement != "root" {
		templateDir = filepath.Join(dir, cleanPlacement)
		relativePath = filepath.ToSlash(filepath.Join(cleanPlacement, cleanName))
	}

	path := filepath.Join(templateDir, cleanName)
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", errors.New("resolved template path escapes custom template directory")
	}

	return path, relativePath, nil
}

func nucleiTemplateResponse(root, path string, info os.FileInfo, placement string, includeContent bool) (dto.NucleiTemplateResponse, error) {
	cleanPlacement, err := normalizeNucleiTemplatePlacement(placement)
	if err != nil {
		return dto.NucleiTemplateResponse{}, err
	}

	relativePath := info.Name()
	if cleanPlacement != "root" {
		relativePath = filepath.ToSlash(filepath.Join(cleanPlacement, info.Name()))
	}

	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			relativePath = filepath.ToSlash(rel)
		}
	}

	resp := dto.NucleiTemplateResponse{
		Name:         info.Name(),
		Placement:    cleanPlacement,
		RelativePath: relativePath,
		Path:         path,
		SizeBytes:    info.Size(),
		UpdatedAt:    info.ModTime(),
	}
	if includeContent {
		content, err := os.ReadFile(path)
		if err != nil {
			return resp, err
		}
		resp.Content = string(content)
	}
	return resp, nil
}

func placementFromRelativePath(relativePath string) string {
	relativePath = filepath.ToSlash(filepath.Clean(relativePath))
	if relativePath == "." || relativePath == "" || strings.HasPrefix(relativePath, "../") {
		return "root"
	}
	parts := strings.Split(relativePath, "/")
	if len(parts) < 2 {
		return "root"
	}
	if _, ok := allowedNucleiTemplatePlacements[parts[0]]; ok {
		return parts[0]
	}
	return "root"
}

func templateNameFromRelativePath(relativePath string) string {
	return filepath.Base(filepath.ToSlash(relativePath))
}

// ListNucleiTemplates returns all custom Nuclei templates stored in NUCLEI_CUSTOM_TEMPLATES_DIR.
func ListNucleiTemplates(c *fiber.Ctx) error {
	dir := nucleiCustomTemplatesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to create custom templates directory",
			"error":   err.Error(),
		})
	}

	templates := make([]dto.NucleiTemplateResponse, 0)
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		name := entry.Name()
		if _, err := validateNucleiTemplateName(name); err != nil {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil
		}

		placement := placementFromRelativePath(rel)
		info, err := entry.Info()
		if err != nil {
			return nil
		}

		resp, err := nucleiTemplateResponse(dir, path, info, placement, false)
		if err != nil {
			return nil
		}
		templates = append(templates, resp)
		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to read custom templates directory",
			"error":   err.Error(),
		})
	}

	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Placement == templates[j].Placement {
			return templates[i].Name < templates[j].Name
		}
		return templates[i].RelativePath < templates[j].RelativePath
	})

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   templates,
		"count":  len(templates),
	})
}

// GetNucleiTemplate returns one custom template including its YAML content.
func GetNucleiTemplate(c *fiber.Ctx) error {
	placement := c.Query("placement", "root")
	path, _, err := nucleiTemplatePath(placement, c.Params("name"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Template not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to read template", "error": err.Error()})
	}
	if info.IsDir() {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Template not found"})
	}

	resp, err := nucleiTemplateResponse(nucleiCustomTemplatesDir(), path, info, placement, true)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to read template", "error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "success", "data": resp})
}

// UpsertNucleiTemplate validates and stores a custom Nuclei template.
func UpsertNucleiTemplate(c *fiber.Ctx) error {
	req := new(dto.UpsertNucleiTemplateRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body", "error": err.Error()})
	}

	placement, err := normalizeNucleiTemplatePlacement(req.Placement)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	path, _, err := nucleiTemplatePath(placement, req.Name)
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

	shouldValidate := true
	if req.Validate != nil {
		shouldValidate = *req.Validate
	}
	if shouldValidate {
		validation := runNucleiTemplateValidation(req.Name, content)
		if !validation.Valid {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":     "error",
				"message":    "Template validation failed",
				"validation": validation,
			})
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to create custom templates directory", "error": err.Error()})
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o600); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to save template", "error": err.Error()})
	}

	info, err := os.Stat(path)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Template saved but metadata could not be read", "error": err.Error()})
	}
	resp, err := nucleiTemplateResponse(nucleiCustomTemplatesDir(), path, info, placement, true)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Template saved but content could not be read", "error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Template saved", "data": resp})
}

// ValidateNucleiTemplate validates posted template content or an already saved custom template.
func ValidateNucleiTemplate(c *fiber.Ctx) error {
	req := new(dto.ValidateNucleiTemplateRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body", "error": err.Error()})
	}

	placement, err := normalizeNucleiTemplatePlacement(req.Placement)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	name := strings.TrimSpace(req.Name)
	content := strings.TrimSpace(req.Content)
	if content == "" {
		if name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Provide either content or an existing template name"})
		}
		path, _, err := nucleiTemplatePath(placement, name)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
		}
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Template not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to read template", "error": err.Error()})
		}
		content = string(b)
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
	placement := c.Query("placement", "root")
	path, _, err := nucleiTemplatePath(placement, c.Params("name"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Template not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to delete template", "error": err.Error()})
	}
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
