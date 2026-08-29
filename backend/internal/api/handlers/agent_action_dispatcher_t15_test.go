package handlers

import (
	"strings"
	"testing"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
)

func TestHunterObjectiveForAction_UsesDescription(t *testing.T) {
	action := models.AgentAction{
		Title:       "Test XSS on httpbin",
		Description: "Probe /anything and /forms/post for reflected XSS via the 'xss' parameter",
	}
	got := hunterObjectiveForAction(action, "fallback suffix")
	if !strings.Contains(got, "Probe /anything") {
		t.Errorf("expected description prefix, got %q", got)
	}
	if !strings.Contains(got, "fallback suffix") {
		t.Errorf("expected default suffix, got %q", got)
	}
}

func TestHunterObjectiveForAction_FallsBackToTitle(t *testing.T) {
	action := models.AgentAction{
		Title:       "title-only",
		Description: "",
	}
	got := hunterObjectiveForAction(action, "default")
	if !strings.Contains(got, "title-only") {
		t.Errorf("expected title fallback, got %q", got)
	}
}

func TestHunterObjectiveForAction_DefaultWhenEmpty(t *testing.T) {
	action := models.AgentAction{}
	got := hunterObjectiveForAction(action, "just default")
	if got != "just default" {
		t.Errorf("expected just default, got %q", got)
	}
}

func TestHunterObjectiveForAction_TrimsWhitespace(t *testing.T) {
	action := models.AgentAction{
		Title:       "  spaced  ",
		Description: "",
	}
	got := hunterObjectiveForAction(action, "default")
	if !strings.Contains(got, "spaced") {
		t.Errorf("expected trimmed title, got %q", got)
	}
}

func TestNewAgentActionTypes_CompileConstants(t *testing.T) {
	// These constants MUST match the wire strings the chat panel
	// emits. If the agent sends a different case, the dispatcher's
	// switch case never matches and the action silently no-ops.
	want := map[string]string{
		"xss":  "test_xss_on_target",
		"sqli": "test_sqli_on_target",
		"idor": "test_idor_on_target",
	}
	got := map[string]string{
		"xss":  string(models.AgentActionTypeTestXSSOnTarget),
		"sqli": string(models.AgentActionTypeTestSQLiOnTarget),
		"idor": string(models.AgentActionTypeTestIDOROnTarget),
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("AgentActionType[%s] = %q, want %q", k, got[k], w)
		}
	}
}
