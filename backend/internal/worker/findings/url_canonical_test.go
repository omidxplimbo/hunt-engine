package findings

import (
	"testing"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
)

func TestCanonicalURLForFindingDropsVolatileParams(t *testing.T) {
	t.Parallel()

	rawA := "https://developerportal.uscellular.com/g/login/auth?org.apache.catalina.filters.CSRF_NONCE=12E6016633FD3B0F5717FC423660EF75&org=%5Bapache.catalina.filters.CSRF_NONCE%3A12E6016633FD3B0F5717FC423660EF75%5D&org.apache.catalina.filters.CSRF_NONCE=6667C2D7673532201F6531EEDF011C80"
	rawB := "https://developerportal.uscellular.com/g/login/auth?org.apache.catalina.filters.CSRF_NONCE=B57E20759290863D0CE1370DEC225044&org=%5Bapache.catalina.filters.CSRF_NONCE%3AB57E20759290863D0CE1370DEC225044%5D&org.apache.catalina.filters.CSRF_NONCE=1E17DA94C6030F37CE599222B4B65AB5"

	want := "https://developerportal.uscellular.com/g/login/auth"

	if got := canonicalURLForFinding(rawA); got != want {
		t.Fatalf("canonical rawA mismatch: got %q want %q", got, want)
	}

	if got := canonicalURLForFinding(rawB); got != want {
		t.Fatalf("canonical rawB mismatch: got %q want %q", got, want)
	}
}

func TestCanonicalURLForFindingKeepsParameterNamesButDropsValues(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"https://example.com/product?id=123":                 "https://example.com/product?id",
		"https://example.com/product?id=456":                 "https://example.com/product?id",
		"https://example.com/search?b=2&a=1":                 "https://example.com/search?a&b",
		"https://example.com:443/search?a=1#section":         "https://example.com/search?a",
		"http://example.com:80//admin///login/?utm_source=x": "http://example.com/admin/login",
	}

	for raw, want := range tests {
		if got := canonicalURLForFinding(raw); got != want {
			t.Fatalf("canonical mismatch for %q: got %q want %q", raw, got, want)
		}
	}
}

func TestURLFindingFingerprintUsesCanonicalURL(t *testing.T) {
	t.Parallel()

	first := models.FoundURL{
		ID:     100,
		Value:  "https://example.com/admin/login?csrf=aaa&utm_source=x",
		Source: "wayback",
	}
	second := models.FoundURL{
		ID:     200,
		Value:  "https://example.com/admin/login?csrf=bbb&utm_campaign=y",
		Source: "wayback",
	}

	firstCandidates := candidatesFromURL(first)
	secondCandidates := candidatesFromURL(second)

	if len(firstCandidates) == 0 || len(secondCandidates) == 0 {
		t.Fatalf("expected URL candidates from admin login URLs")
	}

	firstFingerprint := stableFingerprint(1, firstCandidates[0])
	secondFingerprint := stableFingerprint(1, secondCandidates[0])

	if firstFingerprint != secondFingerprint {
		t.Fatalf("expected canonicalized URL fingerprints to match: %s != %s", firstFingerprint, secondFingerprint)
	}
}

func TestURLEvidenceUsesCanonicalAndSampleRawURL(t *testing.T) {
	t.Parallel()

	foundURL := models.FoundURL{
		ID:     1,
		Value:  "https://example.com/login?csrf=aaa",
		Source: "wayback",
	}

	canonical := canonicalURLForFinding(foundURL.Value)
	evidence := urlEvidence(foundURL, canonical)

	if !containsAny(evidence, []string{"canonical_url=https://example.com/login"}) {
		t.Fatalf("expected canonical URL in evidence, got %q", evidence)
	}

	if !containsAny(evidence, []string{"sample_raw_url=https://example.com/login?csrf=aaa"}) {
		t.Fatalf("expected sample raw URL in evidence, got %q", evidence)
	}
}
