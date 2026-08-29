package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCraftPayloadTool_Name(t *testing.T) {
	c := NewCraftPayloadTool()
	if c.Name() != "craft_payload" {
		t.Errorf("Name() = %q, want craft_payload", c.Name())
	}
}

func TestCraftPayloadTool_Schema(t *testing.T) {
	c := NewCraftPayloadTool()
	schema := c.Schema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no properties: %+v", schema)
	}
	for _, k := range []string{"vector", "target", "observations", "strategy", "max_attempts"} {
		if _, ok := props[k]; !ok {
			t.Errorf("schema missing %q: %+v", k, props)
		}
	}
}

func TestCraftPayloadTool_Execute_MissingRequired(t *testing.T) {
	c := NewCraftPayloadTool()
	ctx := WithLLMConfig(context.Background(), nil)

	cases := []struct {
		name   string
		params map[string]any
	}{
		{"empty", map[string]any{}},
		{"no vector", map[string]any{"target": "https://t", "observations": "x"}},
		{"no target", map[string]any{"vector": "xss", "observations": "x"}},
		{"no observations", map[string]any{"vector": "xss", "target": "https://t"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.Execute(ctx, tc.params)
			if err == nil {
				t.Errorf("Execute should error for %s", tc.name)
			}
		})
	}
}

func TestCraftPayloadTool_Execute_NoConfig(t *testing.T) {
	c := NewCraftPayloadTool()
	_, err := c.Execute(context.Background(), map[string]any{
		"vector":       "xss_reflected",
		"target":       "https://example.com/?q=",
		"observations": "WAF blocks <script>",
	})
	if err == nil {
		t.Fatalf("Execute should error when no LLM config in context")
	}
	if !strings.Contains(err.Error(), "LLM config") {
		t.Errorf("error should mention LLM config: %v", err)
	}
}

func TestCraftPayloadTool_Execute_RejectsBadConfigType(t *testing.T) {
	c := NewCraftPayloadTool()
	ctx := WithLLMConfig(context.Background(), "not-a-config")
	_, err := c.Execute(ctx, map[string]any{
		"vector":       "xss_reflected",
		"target":       "https://example.com/?q=",
		"observations": "WAF blocks <script>",
	})
	if err == nil {
		t.Fatalf("Execute should error when LLM config is wrong type")
	}
	if !errors.Is(err, err) || !strings.Contains(err.Error(), "*llmclient.Config") {
		t.Logf("note: type-assertion error format: %v", err)
	}
}

func TestExtractCraftedPayloads_TopLevelArray(t *testing.T) {
	m := map[string]any{
		"payloads": []any{
			map[string]any{"payload": "<img src=x onerror=alert(1)>", "technique": "tag_in_attr", "why": "no quote needed"},
			map[string]any{"payload": "<svg onload=alert(1)>", "technique": "svg_load", "why": "svg context"},
		},
	}
	got := extractCraftedPayloads(m)
	if len(got) != 2 {
		t.Fatalf("expected 2 payloads, got %d", len(got))
	}
	if got[0].Payload != "<img src=x onerror=alert(1)>" {
		t.Errorf("payload[0] = %q", got[0].Payload)
	}
	if got[0].Technique != "tag_in_attr" {
		t.Errorf("technique[0] = %q", got[0].Technique)
	}
}

func TestExtractCraftedPayloads_ResultsKey(t *testing.T) {
	m := map[string]any{
		"results": []any{
			map[string]any{"payload": "x", "technique": "y", "why": "z"},
		},
	}
	got := extractCraftedPayloads(m)
	if len(got) != 1 {
		t.Errorf("expected 1 payload via 'results' key, got %d", len(got))
	}
}

func TestExtractCraftedPayloads_SkipsEmptyPayloads(t *testing.T) {
	m := map[string]any{
		"payloads": []any{
			map[string]any{"payload": "", "technique": "x"},
			map[string]any{"payload": "valid", "technique": "y"},
		},
	}
	got := extractCraftedPayloads(m)
	if len(got) != 1 {
		t.Errorf("expected 1 valid payload, got %d", len(got))
	}
	if got[0].Payload != "valid" {
		t.Errorf("got %q, want 'valid'", got[0].Payload)
	}
}

func TestExtractCraftedPayloads_UnknownShape(t *testing.T) {
	got := extractCraftedPayloads(map[string]any{"foo": "bar"})
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestBuildCraftPayloadSystemPrompt_ContainsVector(t *testing.T) {
	p := buildCraftPayloadSystemPrompt("xss_reflected", "waf_bypass", 3)
	for _, want := range []string{"xss_reflected", "waf_bypass", "3"} {
		if !strings.Contains(p, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestCraftPayloadTool_JSONOutputShape(t *testing.T) {
	// We can't actually call an LLM in a unit test, but we can
	// verify the JSON encoding of CraftedPayload is what we expect.
	p := []CraftedPayload{
		{Payload: "a", Technique: "t1", Why: "w1"},
		{Payload: "b", Technique: "t2", Why: "w2"},
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Must contain all three fields.
	for _, want := range []string{`"payload":"a"`, `"technique":"t1"`, `"why":"w1"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}
