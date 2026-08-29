package tools

import (
	"context"
	"errors"
	"fmt"
)

// operatorChannelKey is the unexported context key under which the
// AgentLoop stashes the session's *OperatorChannel before invoking a
// tool. Tools that need operator interaction (currently only
// ask_operator) extract it via operatorChannelFromCtx.
type operatorChannelKey struct{}

// WithOperatorChannel returns a derived context carrying the
// OperatorChannel. The AgentLoop calls this just before tool.Execute.
func WithOperatorChannel(ctx context.Context, ch OperatorChannel) context.Context {
	return context.WithValue(ctx, operatorChannelKey{}, ch)
}

// operatorChannelFromCtx extracts the OperatorChannel or returns nil
// if the context was not prepared (legacy / non-session callers).
func operatorChannelFromCtx(ctx context.Context) OperatorChannel {
	v := ctx.Value(operatorChannelKey{})
	if v == nil {
		return nil
	}
	ch, _ := v.(OperatorChannel)
	return ch
}

// OperatorChannel is the read-side interface the ask_operator tool
// uses. Implemented in the hunter package by *OperatorChannel. Kept
// here to avoid a tools → hunter import cycle.
type OperatorChannel interface {
	// Ask blocks until the operator answers the question or the
	// per-question timeout elapses. Returns the operator's answer
	// (possibly empty if the operator submitted a blank response) or
	// an error on context cancellation.
	Ask(ctx context.Context, question string) (string, error)
}

// AskOperatorTool is the tool the LLM calls to ask the operator a
// question. The AgentLoop blocks on this until the operator answers
// (via the WS operator_answer message or the HTTP endpoint) or 5
// minutes elapses (auto-skip with a structured marker).
type AskOperatorTool struct{}

// NewAskOperatorTool returns a fresh instance. Stateless — multiple
// instances are equivalent.
func NewAskOperatorTool() *AskOperatorTool { return &AskOperatorTool{} }

// Name returns the tool's registry name. The LLM sees this in its
// system prompt's tool list.
func (a *AskOperatorTool) Name() string { return "ask_operator" }

// Description is shown to the LLM in the system prompt. The LLM uses
// this to decide when to call the tool.
func (a *AskOperatorTool) Description() string {
	return "Ask the operator a question. Blocks until the operator answers (or 5 minutes → auto-skip with a structured marker). Use this when you need scope confirmation, a credential, or explicit guidance. Do NOT use this for status questions or when the answer is in the skill markdown."
}

// Schema is the JSON schema sent to the LLM. Only one required field
// (question) plus two optional structured fields the LLM may fill to
// give the operator UI richer context.
func (a *AskOperatorTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The exact question you want the operator to answer. Be specific — the operator sees this verbatim.",
				"minLength":   1,
				"maxLength":   2000,
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional free-form context that explains why you are asking. Shown to the operator in the UI but not part of the question itself.",
				"maxLength":   2000,
			},
			"options": map[string]any{
				"type":        "array",
				"description": "Optional list of suggested answers. The UI renders these as clickable chips; the operator can still type a free-form answer.",
				"items":       map[string]any{"type": "string"},
				"maxItems":    8,
			},
		},
		"required": []string{"question"},
	}
}

// Execute runs the tool. Pulls the OperatorChannel out of the
// context (the AgentLoop injects it before calling). If the channel
// is missing (legacy / test context), returns an error string the
// LLM can pattern-match on.
func (a *AskOperatorTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	question, _ := params["question"].(string)
	if question == "" {
		return "", errors.New("ask_operator: 'question' is required")
	}

	ch := operatorChannelFromCtx(ctx)
	if ch == nil {
		return "", errors.New("ask_operator: no operator channel in context (session not attached?)")
	}

	answer, err := ch.Ask(ctx, question)
	if err != nil {
		return "", fmt.Errorf("ask_operator: %w", err)
	}
	return answer, nil
}
