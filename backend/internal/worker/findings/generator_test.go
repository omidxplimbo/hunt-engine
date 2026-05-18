package findings

import (
	"testing"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
)

func TestCandidatesFromAssetDetectsAdminDirectoryServerErrorAndRiskyPorts(t *testing.T) {
	asset := models.Asset{
		ID:           10,
		TargetID:     1,
		Value:        "admin.example.com",
		FinalURL:     "https://admin.example.com/login",
		Title:        "Admin Login - Index of /",
		StatusCode:   503,
		WebServer:    "nginx",
		Technologies: "[\"grafana\"]",
		OpenPorts:    `{"10.0.0.1":[80,443,6379,3306]}`,
	}

	candidates := candidatesFromAsset(asset)

	assertCandidate(t, candidates, "exposed-interface", "medium")
	assertCandidate(t, candidates, "directory-listing", "medium")
	assertCandidate(t, candidates, "server-error", "low")
	assertCandidate(t, candidates, "exposed-service", "high")
	assertCandidate(t, candidates, "exposed-service", "medium")
}

func TestCandidatesFromURLDetectsSensitivePaths(t *testing.T) {
	assetID := uint(42)
	tests := []struct {
		name     string
		url      string
		category string
		severity string
	}{
		{name: "admin login", url: "https://app.example.com/admin/login", category: "sensitive-url", severity: "medium"},
		{name: "config file", url: "https://app.example.com/.env", category: "exposed-config", severity: "high"},
		{name: "version control", url: "https://app.example.com/.git/config", category: "exposed-vcs", severity: "high"},
		{name: "api docs", url: "https://api.example.com/swagger.json", category: "api-documentation", severity: "medium"},
		{name: "debug endpoint", url: "https://app.example.com/actuator/metrics", category: "debug-endpoint", severity: "medium"},
		{name: "backup artifact", url: "https://app.example.com/db-backup.sql", category: "backup-artifact", severity: "high"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			foundURL := models.FoundURL{ID: 100, TargetID: 1, AssetID: &assetID, Value: tc.url, Source: "katana"}
			candidates := candidatesFromURL(foundURL)
			assertCandidate(t, candidates, tc.category, tc.severity)
		})
	}
}

func TestStableFingerprintIsStableAndChangesBySignal(t *testing.T) {
	assetID := uint(10)
	candidate := findingCandidate{AssetID: &assetID, Title: "Possible exposed admin or login interface", Severity: models.FindingSeverityMedium, Category: "exposed-interface", FingerprintKey: "admin-interface"}

	first := stableFingerprint(1, candidate)
	second := stableFingerprint(1, candidate)
	if first == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	if first != second {
		t.Fatalf("expected stable fingerprint, got %q and %q", first, second)
	}

	otherAssetID := uint(11)
	candidate.AssetID = &otherAssetID
	third := stableFingerprint(1, candidate)
	if first == third {
		t.Fatalf("expected different fingerprint for different asset id, got %q", third)
	}
}

func TestRiskyOpenPortsIgnoresNonRiskyPorts(t *testing.T) {
	ports := riskyOpenPorts(`{"example.com":[80,443,6379,9200]}`)
	if len(ports) != 1 {
		t.Fatalf("expected one host with risky ports, got %#v", ports)
	}

	got := ports["example.com"]
	if len(got) != 2 {
		t.Fatalf("expected only risky ports 6379 and 9200, got %#v", got)
	}
	if got[0] != 6379 || got[1] != 9200 {
		t.Fatalf("unexpected risky ports: %#v", got)
	}
}

func assertCandidate(t *testing.T, candidates []findingCandidate, category string, severity string) {
	t.Helper()
	for _, candidate := range candidates {
		if candidate.Category == category && candidate.Severity == severity {
			return
		}
	}
	t.Fatalf("expected candidate category=%s severity=%s in %#v", category, severity, candidates)
}
