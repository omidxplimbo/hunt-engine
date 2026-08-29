package tools

import (
	"context"
	"fmt"

	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/llmclient"
)

// craftPayloadCallLLM is a thin adapter that calls the real
// llmclient.GenerateJSON. Kept in its own file to avoid a hard
// llmclient import at the top of craft_payload.go (and to make the
// adapter easy to mock in tests).
//
// The cfg parameter is what AgentLoop stashes via tools.WithLLMConfig
// — typed as `any` in the call site so the tools package does not
// import llmclient. We type-assert here.
func craftPayloadCallLLM(ctx context.Context, cfg any, systemPrompt string, userPayload map[string]any) (map[string]any, error) {
	if cfg == nil {
		return nil, fmt.Errorf("craft_payload: nil LLM config")
	}
	llmCfg, ok := cfg.(*llmclient.Config)
	if !ok {
		return nil, fmt.Errorf("craft_payload: LLM config is %T, want *llmclient.Config", cfg)
	}
	return llmclient.GenerateJSON(ctx, llmCfg, llmclient.JSONRequest{
		SystemPrompt: systemPrompt,
		UserPayload:  userPayload,
		Temperature:  0.7,
		MaxTokens:    2000,
	})
}
