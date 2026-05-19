package handlers

import (
	"testing"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
)

func TestBuildNucleiTargetTemplateStrategyDetectsPanelAndCMS(t *testing.T) {
	target := models.Target{ID: 42, Name: "Example", RootDomain: "example.com", UseNuclei: true, NucleiProfile: "fast"}
	assets := []models.Asset{
		{
			TargetID:      42,
			Value:         "admin.example.com",
			FinalURL:      "https://admin.example.com/login",
			Title:         "Admin Login",
			WebServer:     "nginx",
			Technologies:  `["WordPress","PHP"]`,
			OpenPorts:     `{"1.2.3.4":[80,443,8080]}`,
			StatusCode:    200,
			ContentLength: 1234,
		},
	}

	strategy := buildNucleiTargetTemplateStrategy(target, assets, true)

	if !strategy.AgentReady {
		t.Fatal("expected agent_ready=true")
	}
	if strategy.ExecuteAutomatically || strategy.SaveAutomatically {
		t.Fatal("strategy must not auto-save or auto-execute")
	}
	if !strategy.AllowedActions.RequiresHumanApproval {
		t.Fatal("strategy must require human approval")
	}
	if strategy.RecommendedProfile != "balanced" {
		t.Fatalf("expected balanced profile, got %q", strategy.RecommendedProfile)
	}
	assertStringInSlice(t, strategy.RecommendedTags, "default-login")
	assertStringInSlice(t, strategy.RecommendedTags, "cms")
	assertStringInSlice(t, strategy.RecommendedPlacements, "fast")
	assertStringInSlice(t, strategy.RecommendedPlacements, "balanced")
	if strategy.SuggestedDraftRequest == nil {
		t.Fatal("expected suggested draft request")
	}
	if strategy.SuggestedDraftRequest.MatcherValue != "Admin Login" {
		t.Fatalf("unexpected matcher value: %q", strategy.SuggestedDraftRequest.MatcherValue)
	}
}

func TestBuildNucleiTargetTemplateStrategyNoLiveAssetsStaysSafe(t *testing.T) {
	target := models.Target{ID: 7, Name: "Empty", RootDomain: "empty.example", UseNuclei: true, NucleiProfile: "fast"}

	strategy := buildNucleiTargetTemplateStrategy(target, nil, false)

	if strategy.RecommendedProfile != "safe" {
		t.Fatalf("expected safe profile for empty live context, got %q", strategy.RecommendedProfile)
	}
	if strategy.SuggestedDraftRequest != nil {
		t.Fatal("did not expect a suggested draft without live assets")
	}
	if strategy.AllowedActions.CanGenerateDraft {
		t.Fatal("draft generation must be unavailable when AI draft flag is disabled")
	}
}

func assertStringInSlice(t *testing.T, values []string, expected string) {
	t.Helper()
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("expected %q in %#v", expected, values)
}
