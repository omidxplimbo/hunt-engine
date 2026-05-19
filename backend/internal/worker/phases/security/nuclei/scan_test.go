package nuclei

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestCustomTemplateArgsSelectsSafeProfileTemplates(t *testing.T) {
	root := t.TempDir()

	writeTestTemplate(t, root, "root.yaml")
	writeTestTemplate(t, root, "shared/shared.yaml")
	writeTestTemplate(t, root, "safe/safe.yaml")
	writeTestTemplate(t, root, "fast/fast.yaml")
	writeTestTemplate(t, root, "balanced/balanced.yaml")
	writeTestTemplate(t, root, "full/full.yaml")

	args := customTemplateArgs(root, "safe")

	assertHasPathSuffix(t, args, "root.yaml")
	assertHasPathSuffix(t, args, "shared")
	assertHasPathSuffix(t, args, "safe")
	assertMissingPathSuffix(t, args, "fast")
	assertMissingPathSuffix(t, args, "balanced")
	assertMissingPathSuffix(t, args, "full")
}

func TestCustomTemplateArgsSelectsFastProfileTemplates(t *testing.T) {
	root := t.TempDir()

	writeTestTemplate(t, root, "root.yaml")
	writeTestTemplate(t, root, "shared/shared.yaml")
	writeTestTemplate(t, root, "safe/safe.yaml")
	writeTestTemplate(t, root, "fast/fast.yaml")
	writeTestTemplate(t, root, "balanced/balanced.yaml")
	writeTestTemplate(t, root, "full/full.yaml")

	args := customTemplateArgs(root, "fast")

	assertHasPathSuffix(t, args, "root.yaml")
	assertHasPathSuffix(t, args, "shared")
	assertHasPathSuffix(t, args, "safe")
	assertHasPathSuffix(t, args, "fast")
	assertMissingPathSuffix(t, args, "balanced")
	assertMissingPathSuffix(t, args, "full")
}

func TestCustomTemplateArgsSelectsBalancedProfileTemplates(t *testing.T) {
	root := t.TempDir()

	writeTestTemplate(t, root, "root.yaml")
	writeTestTemplate(t, root, "fast/fast.yaml")
	writeTestTemplate(t, root, "balanced/balanced.yaml")
	writeTestTemplate(t, root, "full/full.yaml")

	args := customTemplateArgs(root, "balanced")

	assertHasPathSuffix(t, args, "root.yaml")
	assertHasPathSuffix(t, args, "fast")
	assertHasPathSuffix(t, args, "balanced")
	assertMissingPathSuffix(t, args, "full")
}

func TestCustomTemplateArgsSelectsFullProfileTemplates(t *testing.T) {
	root := t.TempDir()

	writeTestTemplate(t, root, "root.yaml")
	writeTestTemplate(t, root, "fast/fast.yaml")
	writeTestTemplate(t, root, "balanced/balanced.yaml")
	writeTestTemplate(t, root, "cves/cve.yaml")
	writeTestTemplate(t, root, "full/full.yaml")
	writeTestTemplate(t, root, "custom/custom.yaml")

	args := customTemplateArgs(root, "full")

	assertHasPathSuffix(t, args, "root.yaml")
	assertHasPathSuffix(t, args, "fast")
	assertHasPathSuffix(t, args, "balanced")
	assertHasPathSuffix(t, args, "cves")
	assertHasPathSuffix(t, args, "full")
	assertHasPathSuffix(t, args, "custom")
}

func TestBuildArgsAddsProfileAwareCustomTemplatePaths(t *testing.T) {
	templatesDir := t.TempDir()
	customDir := t.TempDir()

	writeTestTemplate(t, templatesDir, "builtin.yaml")
	writeTestTemplate(t, customDir, "root.yaml")
	writeTestTemplate(t, customDir, "fast/fast.yaml")
	writeTestTemplate(t, customDir, "full/full.yaml")

	args := buildArgs(Config{TemplatesDir: templatesDir, CustomTemplatesDir: customDir}, "fast", "targets.txt", "out.jsonl")

	assertContainsPair(t, args, "-t", templatesDir)
	assertContainsPair(t, args, "-t", filepath.Join(customDir, "root.yaml"))
	assertContainsPair(t, args, "-t", filepath.Join(customDir, "fast"))
	assertNotContainsPair(t, args, "-t", filepath.Join(customDir, "full"))
}

func writeTestTemplate(t *testing.T, root, relative string) {
	t.Helper()

	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir test template dir: %v", err)
	}

	content := []byte(`id: test-template

info:
  name: test-template
  author: hunt-engine
  severity: info

http:
  - method: GET
    path:
      - "{{BaseURL}}/"
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write test template: %v", err)
	}
}

func assertHasPathSuffix(t *testing.T, values []string, suffix string) {
	t.Helper()

	suffix = filepath.Clean(suffix)
	for _, value := range values {
		if strings.HasSuffix(filepath.Clean(value), suffix) {
			return
		}
	}

	t.Fatalf("expected path suffix %q in %#v", suffix, values)
}

func assertMissingPathSuffix(t *testing.T, values []string, suffix string) {
	t.Helper()

	suffix = filepath.Clean(suffix)
	for _, value := range values {
		if strings.HasSuffix(filepath.Clean(value), suffix) {
			t.Fatalf("did not expect path suffix %q in %#v", suffix, values)
		}
	}
}

func assertNotContainsPair(t *testing.T, args []string, key, value string) {
	t.Helper()
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			t.Fatalf("expected args not to contain %s %s, got %#v", key, value, args)
		}
	}
}
