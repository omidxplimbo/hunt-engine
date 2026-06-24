package handlers

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/auditlog"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	wordliststore "github.com/omidxplimbo/hunt-engine/backend/internal/platform/wordlists"
)

const (
	wordlistImportStatusQueued      = "queued"
	wordlistImportStatusDownloading = "downloading"
	wordlistImportStatusCompleted   = "completed"
	wordlistImportStatusFailed      = "failed"
)

type wordlistImportJobResponse struct {
	ID              uint       `json:"id"`
	UserID          uint       `json:"user_id"`
	WordlistID      *uint      `json:"wordlist_id,omitempty"`
	DisplayName     string     `json:"display_name"`
	SourceURL       string     `json:"source_url"`
	Status          string     `json:"status"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	BytesDownloaded int64      `json:"bytes_downloaded"`
	SizeBytes       int64      `json:"size_bytes"`
	Lines           int64      `json:"lines"`
	SHA256          string     `json:"sha256,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

func ListMyWordlistImportJobs(c *fiber.Ctx) error {
	user, err := currentUser(c)
	if err != nil {
		return err
	}

	var rows []models.UserWordlistImportJob
	if err := database.DB.
		Where("user_id = ?", user.ID).
		Order("created_at desc, id desc").
		Limit(25).
		Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	out := make([]wordlistImportJobResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapWordlistImportJob(&row))
	}

	return c.JSON(fiber.Map{"status": "success", "data": out})
}

func GetMyWordlistImportJob(c *fiber.Ctx) error {
	user, err := currentUser(c)
	if err != nil {
		return err
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid import job id"})
	}

	var row models.UserWordlistImportJob
	if err := database.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&row).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "import job not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": mapWordlistImportJob(&row)})
}

func queueWordlistURLImport(c *fiber.Ctx, user *models.User, req uploadWordlistURLRequest) error {
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "url is required"})
	}

	displayName := strings.TrimSpace(req.Name)
	if displayName == "" {
		parsed, _ := url.Parse(rawURL)
		displayName = strings.TrimSpace(parsed.Query().Get("filename"))
		if displayName == "" {
			displayName = strings.TrimSpace(parsed.Query().Get("name"))
		}
		if displayName == "" {
			displayName = strings.TrimSpace(pathBaseFromURLPath(parsed.Path))
		}
	}

	var err error
	displayName, err = wordliststore.SafeDisplayName(displayName)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	if err := validateWordlistImportURL(rawURL); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	job := models.UserWordlistImportJob{
		UserID:      user.ID,
		DisplayName: displayName,
		SourceURL:   rawURL,
		Status:      wordlistImportStatusQueued,
	}

	if err := database.DB.Create(&job).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &user.ID,
		Action:      "account.wordlist.import_url.queued",
		EntityType:  "user_wordlist_import_job",
		EntityID:    &job.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"user_id":      user.ID,
			"display_name": displayName,
			"source_url":   rawURL,
		},
	})

	go processWordlistURLImportJob(job.ID)

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status": "success",
		"data":   mapWordlistImportJob(&job),
	})
}

func processWordlistURLImportJob(jobID uint) {
	var job models.UserWordlistImportJob
	if err := database.DB.First(&job, jobID).Error; err != nil {
		return
	}

	now := time.Now()
	_ = database.DB.Model(&job).Updates(map[string]interface{}{
		"status":     wordlistImportStatusDownloading,
		"started_at": &now,
	}).Error

	var user models.User
	if err := database.DB.First(&user, job.UserID).Error; err != nil {
		failWordlistImportJob(job.ID, fmt.Errorf("failed to load user: %w", err))
		return
	}

	maxFile, maxTotal, unlimited := wordliststore.EffectiveLimits(user)

	currentTotal, err := wordliststore.CurrentTotalSize(user.ID)
	if err != nil {
		failWordlistImportJob(job.ID, err)
		return
	}

	resp, err := fetchWordlistImportURL(job.SourceURL)
	if err != nil {
		failWordlistImportJob(job.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.ContentLength > 0 {
		if err := wordliststore.CheckQuota(currentTotal, resp.ContentLength, maxFile, maxTotal, unlimited); err != nil {
			failWordlistImportJob(job.ID, err)
			return
		}
	}

	reader := &wordlistImportProgressReader{
		reader:        resp.Body,
		jobID:         job.ID,
		updateEvery:   8 * 1024 * 1024,
		updatePeriod:  2 * time.Second,
		lastUpdatedAt: time.Now(),
	}

	row, err := saveWordlistFromReader(user.ID, job.DisplayName, reader, maxFile, currentTotal, maxTotal, unlimited, "url", job.SourceURL)
	if err != nil {
		failWordlistImportJob(job.ID, err)
		return
	}

	completedAt := time.Now()
	updates := map[string]interface{}{
		"status":           wordlistImportStatusCompleted,
		"completed_at":     &completedAt,
		"wordlist_id":      row.ID,
		"bytes_downloaded": row.SizeBytes,
		"size_bytes":       row.SizeBytes,
		"lines":            row.Lines,
		"sha256":           row.SHA256,
		"storage_path":     row.StoragePath,
		"error_message":    "",
	}

	_ = database.DB.Model(&models.UserWordlistImportJob{}).Where("id = ?", job.ID).Updates(updates).Error

	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &user.ID,
		Action:      "account.wordlist.import_url.completed",
		EntityType:  "user_wordlist_import_job",
		EntityID:    &job.ID,
		Metadata: map[string]interface{}{
			"user_id":     user.ID,
			"wordlist_id": row.ID,
			"name":        row.DisplayName,
			"size_bytes":  row.SizeBytes,
			"lines":       row.Lines,
		},
	})
}

func failWordlistImportJob(jobID uint, err error) {
	msg := "import failed"
	if err != nil {
		msg = err.Error()
	}

	completedAt := time.Now()
	_ = database.DB.Model(&models.UserWordlistImportJob{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":        wordlistImportStatusFailed,
			"error_message": msg,
			"completed_at":  &completedAt,
		}).Error
}

type wordlistImportProgressReader struct {
	reader        io.Reader
	jobID         uint
	bytesRead     int64
	lastReported  int64
	updateEvery   int64
	updatePeriod  time.Duration
	lastUpdatedAt time.Time
}

func (r *wordlistImportProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.bytesRead += int64(n)
		if r.shouldReport() {
			r.report()
		}
	}
	if err == io.EOF {
		r.report()
	}
	return n, err
}

func (r *wordlistImportProgressReader) shouldReport() bool {
	if r.updateEvery > 0 && r.bytesRead-r.lastReported >= r.updateEvery {
		return true
	}
	return time.Since(r.lastUpdatedAt) >= r.updatePeriod
}

func (r *wordlistImportProgressReader) report() {
	r.lastReported = r.bytesRead
	r.lastUpdatedAt = time.Now()
	_ = database.DB.Model(&models.UserWordlistImportJob{}).
		Where("id = ?", r.jobID).
		Update("bytes_downloaded", r.bytesRead).Error
}

func validateWordlistImportURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are allowed")
	}

	if err := validatePublicHost(parsed.Hostname()); err != nil {
		return err
	}

	return nil
}

func fetchWordlistImportURL(rawURL string) (*http.Response, error) {
	if err := validateWordlistImportURL(rawURL); err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if err := validatePublicHost(host); err != nil {
				return nil, err
			}
			return (&net.Dialer{
				Timeout:   20 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext(ctx, network, address)
		},
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if err := validateWordlistImportURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}
	req.Header.Set("User-Agent", "hunt-engine-wordlist-import/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("url returned status %d", resp.StatusCode)
	}

	return resp, nil
}

func pathBaseFromURLPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "wordlist.txt"
	}
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part != "" {
			return part
		}
	}
	return "wordlist.txt"
}

func mapWordlistImportJob(row *models.UserWordlistImportJob) wordlistImportJobResponse {
	if row == nil {
		return wordlistImportJobResponse{}
	}

	return wordlistImportJobResponse{
		ID:              row.ID,
		UserID:          row.UserID,
		WordlistID:      row.WordlistID,
		DisplayName:     row.DisplayName,
		SourceURL:       row.SourceURL,
		Status:          row.Status,
		ErrorMessage:    row.ErrorMessage,
		BytesDownloaded: row.BytesDownloaded,
		SizeBytes:       row.SizeBytes,
		Lines:           row.Lines,
		SHA256:          row.SHA256,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		StartedAt:       row.StartedAt,
		CompletedAt:     row.CompletedAt,
	}
}
