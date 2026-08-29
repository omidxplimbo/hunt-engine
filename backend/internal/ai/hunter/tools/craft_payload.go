package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// llmConfigKey is the unexported context key under which the
// AgentLoop stashes the LLM config before invoking a tool that
// needs to call the LLM itself (currently only craft_payload). The
// value is the *llmclient.Config, but we keep it as `any` to avoid
// an import cycle (tools → llmclient would force llmclient → tools).
type llmConfigKey struct{}

// WithLLMConfig returns a derived context carrying the LLM config
// the tool needs to make its own LLM calls. The AgentLoop calls this
// just before tool.Execute.
func WithLLMConfig(ctx context.Context, cfg any) context.Context {
	return context.WithValue(ctx, llmConfigKey{}, cfg)
}

// llmConfigFromCtx extracts the LLM config or returns nil if the
// context was not prepared.
func llmConfigFromCtx(ctx context.Context) any {
	return ctx.Value(llmConfigKey{})
}

// CraftPayloadTool is the tool the LLM calls when it wants to
// generate custom exploit payloads for a specific endpoint based on
// observed response patterns. The tool itself is RiskLow — it does
// NOT execute the payload; the LLM iterates with http / browser
// calls in subsequent turns to verify.
//
// Output is a JSON list of {payload, technique, why} entries. The
// LLM can pick the best candidate(s) and feed them to http / browser.
type CraftPayloadTool struct{}

// NewCraftPayloadTool returns a fresh instance. Stateless.
func NewCraftPayloadTool() *CraftPayloadTool { return &CraftPayloadTool{} }

// Name returns the registry name. The LLM sees this in its system
// prompt's tool list.
func (c *CraftPayloadTool) Name() string { return "craft_payload" }

// Description is shown to the LLM. The wording invites creative use
// without leaking implementation details.
func (c *CraftPayloadTool) Description() string {
	return "Generate 1..N context-specific exploit payloads for a target endpoint based on the response you just saw. Use for WAF bypass, multi-layer encoding, mutation chains, or variants the registry cannot enumerate. Does NOT execute the payload — call http/browser in the next turn to verify each candidate."
}

// Schema is the JSON schema sent to the LLM. Strict typing so the
// LLM produces predictable input.
func (c *CraftPayloadTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vector": map[string]any{
				"type": "string",
				"enum": []string{
					"xss_reflected", "xss_stored", "xss_dom",
					"sqli_error", "sqli_blind", "sqli_time", "sqli_union",
					"ssti", "cmdi", "lfi", "ssrf", "idor",
					"auth_bypass", "open_redirect", "generic",
				},
				"description": "The vulnerability class to craft a payload for.",
			},
			"target": map[string]any{
				"type":        "string",
				"description": "The full URL or identifier (e.g. 'https://target.com/search?q=') where the payload will be tested.",
				"minLength":   1,
				"maxLength":   2000,
			},
			"observations": map[string]any{
				"type":        "string",
				"description": "What did the previous response look like? E.g. 'WAF returns 403 on <script> but encodes single quotes as &apos;'.",
				"maxLength":   4000,
			},
			"strategy": map[string]any{
				"type":        "string",
				"enum":        []string{"waf_bypass", "multi_layer", "mutation_chain", "time_based", "context_aware", "generic"},
				"default":     "generic",
				"description": "The mutation strategy to apply.",
			},
			"max_attempts": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     10,
				"default":     3,
				"description": "How many candidate payloads to return.",
			},
		},
		"required": []string{"vector", "target", "observations"},
	}
}

// CraftedPayload is the per-candidate output schema. Exposed so
// tests + the LLM (via the embedded JSON example) share the same
// shape.
type CraftedPayload struct {
	Payload   string `json:"payload"`
	Technique string `json:"technique"`
	Why       string `json:"why"`
}

// Execute runs the tool. Pulls the LLM config out of context,
// crafts a focused system prompt + user payload, calls the LLM via
// llmclient.GenerateJSON, and returns the JSON string. The LLM has
// already structured the output as a list of {payload, technique, why}.
func (c *CraftPayloadTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	vector, _ := params["vector"].(string)
	target, _ := params["target"].(string)
	observations, _ := params["observations"].(string)
	if vector == "" || target == "" || observations == "" {
		return "", errors.New("craft_payload: vector, target, and observations are all required")
	}

	strategy, _ := params["strategy"].(string)
	if strategy == "" {
		strategy = "generic"
	}

	maxAttempts := 3
	if v, ok := params["max_attempts"]; ok {
		switch n := v.(type) {
		case int:
			maxAttempts = n
		case int64:
			maxAttempts = int(n)
		case float64:
			maxAttempts = int(n)
		}
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if maxAttempts > 10 {
		maxAttempts = 10
	}

	cfg := llmConfigFromCtx(ctx)
	if cfg == nil {
		return "", errors.New("craft_payload: no LLM config in context (session not attached?)")
	}

	systemPrompt := buildCraftPayloadSystemPrompt(vector, strategy, maxAttempts)
	userPayload := map[string]interface{}{
		"target":       target,
		"observations": observations,
		"vector":       vector,
		"strategy":     strategy,
		"max_attempts": maxAttempts,
	}

	// Make the LLM call through the shared llmclient.GenerateJSON.
	// We re-import the function indirectly via a thin adapter
	// interface so tools does not need a hard import on llmclient.
	result, err := craftPayloadCallLLM(ctx, cfg, systemPrompt, userPayload)
	if err != nil {
		return "", fmt.Errorf("craft_payload: LLM call failed: %w", err)
	}

	// Normalize the LLM's response. It may return either a top-level
	// array or a map with a "payloads" key. We accept both.
	normalized := extractCraftedPayloads(result)
	if len(normalized) == 0 {
		// Fallback: if the LLM returned a string, pass it through.
		if s, ok := result["response"].(string); ok {
			return s, nil
		}
		return "", errors.New("craft_payload: LLM returned no payloads")
	}
	if len(normalized) > maxAttempts {
		normalized = normalized[:maxAttempts]
	}
	out, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("craft_payload: marshal: %w", err)
	}
	return string(out), nil
}

// buildCraftPayloadSystemPrompt constructs the focused system prompt
// for the inner LLM call. The wording is deliberately tight so the
// model returns a strict JSON array, not a long explanation.
func buildCraftPayloadSystemPrompt(vector, strategy string, maxAttempts int) string {
	var sb strings.Builder
	sb.WriteString("You are an exploit payload generator. Given a target, an observation of its response, ")
	sb.WriteString("a vulnerability vector, a mutation strategy, and a max_attempts count, you produce ")
	sb.WriteString(fmt.Sprintf("up to %d candidate payloads.\n\n", maxAttempts))
	sb.WriteString("RULES:\n")
	sb.WriteString("- Output a JSON array of objects. Each object has three fields:\n")
	sb.WriteString("  - payload: the exact string to inject (do not URL-encode it; the caller will encode)\n")
	sb.WriteString("  - technique: a short name of the technique (e.g. 'unicode_normalization', 'double_url_encode', 'tag_in_tag')\n")
	sb.WriteString("  - why: one sentence explaining why this should bypass the observed filtering\n")
	sb.WriteString("- The payloads must be syntactically valid for the vector (e.g. valid HTML/JS for xss, valid SQL for sqli).\n")
	sb.WriteString("- Avoid payloads that obviously won't work (e.g. '<script>' for a site that already escapes angle brackets).\n")
	sb.WriteString("- If observations are sparse, prefer widely-known effective payloads over exotic ones.\n")
	sb.WriteString("- Do NOT include markdown fences. Output the raw JSON array only.\n\n")
	sb.WriteString(fmt.Sprintf("Vector: %s\nStrategy: %s\nMax attempts: %d\n", vector, strategy, maxAttempts))
	return sb.String()
}

// extractCraftedPayloads normalizes the LLM response into a slice
// of CraftedPayload. Accepts {payloads: [...]} or a top-level array.
func extractCraftedPayloads(m map[string]any) []CraftedPayload {
	if arr, ok := m["payloads"].([]any); ok {
		return coercePayloads(arr)
	}
	if arr, ok := m["results"].([]any); ok {
		return coercePayloads(arr)
	}
	// Sometimes the LLM wraps the array in a single key matching the
	// vector name. Try the obvious ones.
	for _, k := range []string{"xss", "sqli", "ssti", "payloads"} {
		if arr, ok := m[k].([]any); ok {
			return coercePayloads(arr)
		}
	}
	return nil
}

func coercePayloads(arr []any) []CraftedPayload {
	out := make([]CraftedPayload, 0, len(arr))
	for _, raw := range arr {
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		payload, _ := obj["payload"].(string)
		technique, _ := obj["technique"].(string)
		why, _ := obj["why"].(string)
		if payload == "" {
			continue
		}
		out = append(out, CraftedPayload{
			Payload:   payload,
			Technique: technique,
			Why:       why,
		})
	}
	return out
}
