package handlers

import (
	"strings"
	"testing"

	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
)

func TestBuildNucleiTemplateDraftProducesDraftOnlyTemplate(t *testing.T) {
	content, name, err := buildNucleiTemplateDraft(&dto.GenerateNucleiTemplateDraftRequest{
		Name:         "hunt-ai-draft-test.yaml",
		Title:        "Hunt AI Draft Test",
		Description:  "Draft-only unit test.",
		Severity:     "low",
		Tags:         []string{"exposure", "panel"},
		Method:       "GET",
		Path:         "/",
		MatcherType:  "word",
		MatcherPart:  "body",
		MatcherValue: "HUNT_NUCLEI_TEST_2026_05_19",
	})
	if err != nil {
		t.Fatalf("expected draft generation to succeed: %v", err)
	}
	if name != "hunt-ai-draft-test.yaml" {
		t.Fatalf("unexpected name: %s", name)
	}

	required := []string{
		"id: hunt-ai-draft-test",
		"hunt_engine_draft",
		"requires_human_review",
		"severity: low",
		"tags: exposure,panel",
		"type: word",
		"HUNT_NUCLEI_TEST_2026_05_19",
	}
	for _, fragment := range required {
		if !strings.Contains(content, fragment) {
			t.Fatalf("expected draft content to contain %q; content:\n%s", fragment, content)
		}
	}
}

func TestBuildNucleiTemplateDraftRejectsInvalidSeverity(t *testing.T) {
	_, _, err := buildNucleiTemplateDraft(&dto.GenerateNucleiTemplateDraftRequest{
		Name:         "bad.yaml",
		Severity:     "urgent",
		Method:       "GET",
		MatcherType:  "word",
		MatcherValue: "marker",
	})
	if err == nil {
		t.Fatal("expected invalid severity to fail")
	}
}
