package screenshot

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var safeNameRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func CaptureFreshAsset(userID uint, targetID uint, assetValue string, pageURL string) (string, error) {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		pageURL = "https://" + strings.TrimSpace(assetValue)
	}

	if !isHTTPURL(pageURL) {
		return "", fmt.Errorf("invalid screenshot URL: %s", pageURL)
	}

	browser := findBrowser()
	if browser == "" {
		return "", fmt.Errorf("no chromium-compatible browser found")
	}

	dir := filepath.Join(
		"/data/screenshots",
		"users",
		fmt.Sprintf("%d", userID),
		"targets",
		fmt.Sprintf("%d", targetID),
		"fresh-assets",
	)

	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}

	name := safeName(assetValue)
	if name == "" {
		name = "asset"
	}

	output := filepath.Join(dir, fmt.Sprintf("%s-%d.png", name, time.Now().UnixNano()))

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		browser,
		"--headless",
		"--no-sandbox",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--ignore-certificate-errors",
		"--window-size=1365,768",
		"--screenshot="+output,
		pageURL,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(output)
		return "", fmt.Errorf("capture screenshot: %w: %s", err, strings.TrimSpace(string(out)))
	}

	if info, err := os.Stat(output); err != nil || info.Size() == 0 {
		_ = os.Remove(output)
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("empty screenshot")
	}

	return output, nil
}

func findBrowser() string {
	for _, candidate := range []string{
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
		"/usr/bin/google-chrome",
		"chromium-browser",
		"chromium",
		"google-chrome",
	} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}

	return ""
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}

	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func safeName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, ".")
	value = safeNameRegex.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-._")

	if len(value) > 120 {
		value = value[:120]
	}

	return value
}
