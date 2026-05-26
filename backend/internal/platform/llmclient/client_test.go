package llmclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateTargetNarrativeOpenAICompatibleSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{
							"executive_summary":"Executive narrative from mock LLM.",
							"customer_summary":"Customer-safe summary from mock LLM.",
							"remediation_plan":["Validate exposed interfaces manually.","Prioritize confirmed exploitable findings."],
							"report_notes":["Coverage gaps are not vulnerabilities."],
							"validation_notes":["Check authentication and access control on exposed interfaces."]
						}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	cfg := &Config{
		Provider: "custom",
		APIKey:   "test-key",
		BaseURL:  server.URL,
		Model:    "mock-model",
	}

	snapshot := map[string]interface{}{
		"target": map[string]interface{}{
			"root_domain": "example.com",
		},
	}

	deterministic := map[string]interface{}{
		"risk_score":          39,
		"risk_level":          "medium",
		"methodology_version": "target-risk-v2-commercial-guardrails",
	}

	out, err := GenerateTargetNarrative(context.Background(), cfg, snapshot, deterministic)
	if err != nil {
		t.Fatalf("GenerateTargetNarrative returned error: %v", err)
	}

	if out["llm_assisted"] != true {
		t.Fatalf("llm_assisted = %v, want true", out["llm_assisted"])
	}
	if out["llm_provider"] != "custom" {
		t.Fatalf("llm_provider = %v, want custom", out["llm_provider"])
	}
	if out["llm_model"] != "mock-model" {
		t.Fatalf("llm_model = %v, want mock-model", out["llm_model"])
	}
	if out["executive_summary"] == "" {
		t.Fatalf("executive_summary was empty")
	}

	if _, exists := out["risk_score"]; exists {
		t.Fatalf("narrative output must not contain risk_score")
	}
	if _, exists := out["risk_level"]; exists {
		t.Fatalf("narrative output must not contain risk_level")
	}
}
