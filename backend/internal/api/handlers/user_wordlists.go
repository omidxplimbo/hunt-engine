package handlers

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/auditlog"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	wordliststore "github.com/omidxplimbo/hunt-engine/backend/internal/platform/wordlists"
	"gorm.io/gorm"
)

type wordlistResponse struct {
	ID          uint       `json:"id"`
	UserID      uint       `json:"user_id"`
	Name        string     `json:"name"`
	SizeBytes   int64      `json:"size_bytes"`
	SHA256      string     `json:"sha256"`
	Lines       int64      `json:"lines"`
	Source      string     `json:"source"`
	SourceURL   string     `json:"source_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	PurednsPath string     `json:"puredns_path"`
}

type wordlistListResponse struct {
	Wordlists             []wordlistResponse `json:"wordlists"`
	CurrentTotalSizeBytes int64              `json:"current_total_size_bytes"`
	MaxFileSizeBytes      int64              `json:"max_file_size_bytes"`
	MaxTotalSizeBytes     int64              `json:"max_total_size_bytes"`
	Unlimited             bool               `json:"unlimited"`
}

type uploadWordlistURLRequest struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

func GetMyWordlists(c *fiber.Ctx) error {
	user, err := currentUser(c)
	if err != nil {
		return err
	}

	var rows []models.UserWordlist
	if err := database.DB.
		Where("user_id = ?", user.ID).
		Order("created_at desc, id desc").
		Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	currentTotal, err := wordliststore.CurrentTotalSize(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	maxFile, maxTotal, unlimited := wordliststore.EffectiveLimits(*user)

	return c.JSON(fiber.Map{
		"status": "success",
		"data": wordlistListResponse{
			Wordlists:             mapWordlists(rows),
			CurrentTotalSizeBytes: currentTotal,
			MaxFileSizeBytes:      maxFile,
			MaxTotalSizeBytes:     maxTotal,
			Unlimited:             unlimited,
		},
	})
}

func UploadMyWordlistFile(c *fiber.Ctx) error {
	user, err := currentUser(c)
	if err != nil {
		return err
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "file is required"})
	}

	displayName, err := wordliststore.SafeDisplayName(fileHeader.Filename)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	maxFile, maxTotal, unlimited := wordliststore.EffectiveLimits(*user)

	currentTotal, err := wordliststore.CurrentTotalSize(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	if err := wordliststore.CheckQuota(currentTotal, fileHeader.Size, maxFile, maxTotal, unlimited); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to read uploaded file"})
	}
	defer f.Close()

	row, err := saveWordlistFromReader(user.ID, displayName, f, maxFile, currentTotal, maxTotal, unlimited, "file", "")
	if err != nil {
		return wordlistUploadError(c, err)
	}

	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &user.ID,
		Action:      "account.wordlist.upload",
		EntityType:  "user_wordlist",
		EntityID:    &row.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"user_id":    user.ID,
			"name":       row.DisplayName,
			"size_bytes": row.SizeBytes,
			"source":     row.Source,
		},
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "success",
		"data":   mapWordlist(row),
	})
}

func UploadMyWordlistURL(c *fiber.Ctx) error {
	user, err := currentUser(c)
	if err != nil {
		return err
	}

	req := uploadWordlistURLRequest{}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid request body"})
	}

	return queueWordlistURLImport(c, user, req)
}

func DownloadMyWordlist(c *fiber.Ctx) error {
	user, err := currentUser(c)
	if err != nil {
		return err
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid wordlist id"})
	}

	var row models.UserWordlist
	if err := database.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&row).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "wordlist not found"})
	}

	if strings.TrimSpace(row.StoragePath) == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "wordlist file not found"})
	}

	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &user.ID,
		Action:      "account.wordlist.download",
		EntityType:  "user_wordlist",
		EntityID:    &row.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"user_id": user.ID,
			"name":    row.DisplayName,
		},
	})

	return c.Download(row.StoragePath, row.DisplayName)
}

func DeleteMyWordlist(c *fiber.Ctx) error {
	user, err := currentUser(c)
	if err != nil {
		return err
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid wordlist id"})
	}

	var row models.UserWordlist
	if err := database.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&row).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "wordlist not found"})
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	_ = wordliststore.RemoveFile(row.StoragePath)

	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &user.ID,
		Action:      "account.wordlist.delete",
		EntityType:  "user_wordlist",
		EntityID:    &row.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"user_id": user.ID,
			"name":    row.DisplayName,
		},
	})

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Wordlist deleted",
	})
}

func currentUser(c *fiber.Ctx) (*models.User, error) {
	uid, err := currentUserID(c)
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
	}

	return &user, nil
}

func parseUintParam(c *fiber.Ctx, name string) (uint, error) {
	raw := strings.TrimSpace(c.Params(name))
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return uint(n), nil
}

func saveWordlistFromReader(
	userID uint,
	displayName string,
	reader io.Reader,
	maxFile int64,
	currentTotal int64,
	maxTotal int64,
	unlimited bool,
	source string,
	sourceURL string,
) (*models.UserWordlist, error) {
	stored, err := wordliststore.SaveReader(userID, displayName, reader, maxFile)
	if err != nil {
		return nil, err
	}

	if err := wordliststore.CheckQuota(currentTotal, stored.SizeBytes, maxFile, maxTotal, unlimited); err != nil {
		_ = wordliststore.RemoveFile(stored.Path)
		return nil, err
	}

	row := models.UserWordlist{
		UserID:      userID,
		DisplayName: stored.DisplayName,
		StoredName:  stored.StoredName,
		StoragePath: stored.Path,
		SizeBytes:   stored.SizeBytes,
		SHA256:      stored.SHA256,
		Lines:       stored.Lines,
		Source:      source,
		SourceURL:   sourceURL,
	}

	if err := database.DB.Create(&row).Error; err != nil {
		_ = wordliststore.RemoveFile(stored.Path)
		return nil, err
	}

	return &row, nil
}

func wordlistUploadError(c *fiber.Ctx, err error) error {
	if err == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "upload failed"})
	}

	msg := err.Error()
	status := fiber.StatusBadRequest

	return c.Status(status).JSON(fiber.Map{
		"status":  "error",
		"message": msg,
	})
}

func mapWordlists(rows []models.UserWordlist) []wordlistResponse {
	out := make([]wordlistResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapWordlist(&row))
	}
	return out
}

func mapWordlist(row *models.UserWordlist) wordlistResponse {
	if row == nil {
		return wordlistResponse{}
	}

	return wordlistResponse{
		ID:          row.ID,
		UserID:      row.UserID,
		Name:        row.DisplayName,
		SizeBytes:   row.SizeBytes,
		SHA256:      row.SHA256,
		Lines:       row.Lines,
		Source:      row.Source,
		SourceURL:   row.SourceURL,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		LastUsedAt:  row.LastUsedAt,
		PurednsPath: fmt.Sprintf("user:%d/%s", row.UserID, row.DisplayName),
	}
}

func safeFetchWordlistURL(rawURL string, maxBytes int64) (*http.Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("only http and https URLs are allowed")
	}

	if err := validatePublicHost(parsed.Hostname()); err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 45 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}

				ip, err := resolvePublicIP(host)
				if err != nil {
					return nil, err
				}

				dialer := &net.Dialer{Timeout: 15 * time.Second}
				return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			},
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirected to unsupported scheme")
			}
			return validatePublicHost(req.URL.Hostname())
		},
	}

	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}
	req.Header.Set("User-Agent", "HuntEngine-WordlistFetcher/1.0")
	req.Header.Set("Accept", "text/plain,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch url: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("url returned status %d", resp.StatusCode)
	}

	if maxBytes > 0 && resp.ContentLength > maxBytes {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("file exceeds max file size limit")
	}

	return resp, nil
}

func validatePublicHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("url host is required")
	}

	_, err := resolvePublicIP(host)
	return err
}

func resolvePublicIP(host string) (net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve url host")
	}

	for _, ip := range ips {
		if isPublicIP(ip) {
			return ip, nil
		}
	}

	return nil, fmt.Errorf("url host resolved to a private or unsafe address")
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}

	return true
}

func openMultipartFile(header *multipart.FileHeader) (multipart.File, error) {
	if header == nil {
		return nil, errors.New("file is required")
	}
	return header.Open()
}
