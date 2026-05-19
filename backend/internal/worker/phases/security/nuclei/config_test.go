package nuclei

import (
	"testing"
	"time"
)

func TestNormalizeProfile(t *testing.T) {
	cases := map[string]string{
		"":           "safe",
		"SAFE":       "safe",
		"exposure":   "exposure",
		"misconfig":  "misconfig",
		"cves-light": "cves-light",
		"custom":     "custom",
		"full":       "full",
		"unknown":    "safe",
	}

	for input, want := range cases {
		if got := NormalizeProfile(input); got != want {
			t.Fatalf("NormalizeProfile(%q)=%q want %q", input, got, want)
		}
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("NUCLEI_TIMEOUT_SECONDS", "")
	t.Setenv("NUCLEI_RATE_LIMIT", "")
	t.Setenv("NUCLEI_CONCURRENCY", "")
	t.Setenv("NUCLEI_BULK_SIZE", "")
	t.Setenv("NUCLEI_TEMPLATES_DIR", "")
	t.Setenv("NUCLEI_CUSTOM_TEMPLATES_DIR", "")
	t.Setenv("NUCLEI_DEFAULT_PROFILE", "")
	t.Setenv("NUCLEI_ALLOW_AI_TEMPLATES", "")

	cfg := LoadConfig()
	if cfg.Timeout != time.Duration(DefaultTimeoutSeconds)*time.Second {
		t.Fatalf("unexpected timeout: %s", cfg.Timeout)
	}
	if cfg.RateLimit != DefaultRateLimit || cfg.Concurrency != DefaultConcurrency || cfg.BulkSize != DefaultBulkSize {
		t.Fatalf("unexpected numeric defaults: %+v", cfg)
	}
	if cfg.TemplatesDir != DefaultTemplatesDir || cfg.CustomTemplatesDir != DefaultCustomDir {
		t.Fatalf("unexpected template dirs: %+v", cfg)
	}
	if cfg.DefaultProfile != DefaultProfile {
		t.Fatalf("unexpected profile: %s", cfg.DefaultProfile)
	}
	if cfg.AllowAITemplates {
		t.Fatalf("AI templates should be disabled by default")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("NUCLEI_TIMEOUT_SECONDS", "42")
	t.Setenv("NUCLEI_RATE_LIMIT", "7")
	t.Setenv("NUCLEI_CONCURRENCY", "3")
	t.Setenv("NUCLEI_BULK_SIZE", "9")
	t.Setenv("NUCLEI_TEMPLATES_DIR", "/templates")
	t.Setenv("NUCLEI_CUSTOM_TEMPLATES_DIR", "/custom")
	t.Setenv("NUCLEI_DEFAULT_PROFILE", "exposure")
	t.Setenv("NUCLEI_ALLOW_AI_TEMPLATES", "true")

	cfg := LoadConfig()
	if cfg.Timeout != 42*time.Second || cfg.RateLimit != 7 || cfg.Concurrency != 3 || cfg.BulkSize != 9 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.TemplatesDir != "/templates" || cfg.CustomTemplatesDir != "/custom" {
		t.Fatalf("unexpected dirs: %+v", cfg)
	}
	if cfg.DefaultProfile != "exposure" || !cfg.AllowAITemplates {
		t.Fatalf("unexpected profile/AI: %+v", cfg)
	}
}
