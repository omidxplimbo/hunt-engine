package nuclei

import (
	"encoding/json"
	"testing"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
)

func TestNormalizeProfileSupportsFrontendAliases(t *testing.T) {
	cases := map[string]string{
		"FAST":     "fast",
		"balanced": "balanced",
		"full":     "full",
		"nope":     "safe",
	}
	for input, want := range cases {
		if got := NormalizeProfile(input); got != want {
			t.Fatalf("NormalizeProfile(%q)=%q want %q", input, got, want)
		}
	}
}

func TestProfileArgsAreConservativeByDefault(t *testing.T) {
	args := profileArgs("safe")
	assertContainsPair(t, args, "-severity", "medium,high,critical")
	assertContainsPair(t, args, "-exclude-tags", "dos,bruteforce,brute-force,intrusive,destructive,fuzz")
}

func TestProfileArgsFullKeepsAllSeverities(t *testing.T) {
	args := profileArgs("full")
	assertContainsPair(t, args, "-severity", "info,low,medium,high,critical")
}

func TestNormalizeURLTargetKeepsOnlyHTTPURLs(t *testing.T) {
	if got := normalizeURLTarget("https://app.example.com/a#frag"); got != "https://app.example.com/a" {
		t.Fatalf("unexpected normalized URL: %q", got)
	}
	if got := normalizeURLTarget("app.example.com"); got != "" {
		t.Fatalf("expected host without scheme to be skipped, got %q", got)
	}
	if got := normalizeURLTarget("ftp://app.example.com"); got != "" {
		t.Fatalf("expected non-http scheme to be skipped, got %q", got)
	}
}

func TestFingerprintForResultIsStableAndTemplateScoped(t *testing.T) {
	row := resultRow{TemplateID: "tech-detect", MatchedAt: "https://app.example.com"}
	first := fingerprintForResult(1, row)
	second := fingerprintForResult(1, row)
	if first == "" || first != second {
		t.Fatalf("expected stable non-empty fingerprint, got %q and %q", first, second)
	}
	row.TemplateID = "other"
	if first == fingerprintForResult(1, row) {
		t.Fatalf("expected fingerprint to change when template id changes")
	}
}

func TestNormalizeSeverity(t *testing.T) {
	if got := normalizeSeverity("critical"); got != models.FindingSeverityCritical {
		t.Fatalf("unexpected severity: %s", got)
	}
	if got := normalizeSeverity("unknown"); got != models.FindingSeverityInfo {
		t.Fatalf("unexpected fallback severity: %s", got)
	}
}

func TestTagsFromRawSupportsStringAndArray(t *testing.T) {
	arr, _ := json.Marshal([]string{"cve", "rce"})
	if got := tagsFromRaw(arr); len(got) != 2 || got[0] != "cve" || got[1] != "rce" {
		t.Fatalf("unexpected array tags: %#v", got)
	}
	str, _ := json.Marshal("exposure, panel")
	if got := tagsFromRaw(str); len(got) != 2 || got[0] != "exposure" || got[1] != "panel" {
		t.Fatalf("unexpected string tags: %#v", got)
	}
}

func assertContainsPair(t *testing.T, args []string, key, value string) {
	t.Helper()
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return
		}
	}
	t.Fatalf("expected args to contain %s %s, got %#v", key, value, args)
}
