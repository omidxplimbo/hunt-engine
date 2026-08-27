package hunter

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter/tools"
)

// approvalCollector is a test progress callback that captures every
// emitted AgentEvent so tests can assert the approval_required event
// carried the right action_id and masked params.
type approvalCollector struct {
	mu     sync.Mutex
	events []AgentEvent
}

func (c *approvalCollector) callback(ev AgentEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
}

func (c *approvalCollector) snapshot() []AgentEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]AgentEvent, len(c.events))
	copy(out, c.events)
	return out
}

func TestPolicy_DefaultRiskLevels(t *testing.T) {
	p := tools.DefaultPolicy
	cases := []struct {
		tool string
		want tools.RiskLevel
	}{
		{"http", tools.RiskLow},
		{"browser", tools.RiskMedium},
		{"proxy", tools.RiskMedium},
		{"shell", tools.RiskHigh},
		{"unknown-tool", tools.RiskHigh}, // fallback: assume high
	}
	for _, c := range cases {
		if got := p.RiskFor(c.tool); got != c.want {
			t.Errorf("RiskFor(%q) = %q, want %q", c.tool, got, c.want)
		}
	}
	if !p.RequiresApproval("shell") {
		t.Errorf("shell should require approval")
	}
	if p.RequiresApproval("http") {
		t.Errorf("http should NOT require approval")
	}
}

func TestMaskSensitiveParams_RecursesAndHides(t *testing.T) {
	in := map[string]any{
		"url":          "http://example.com",
		"Authorization": "Bearer secret-123",
		"cookie":       "session=abc",
		"api_key":      "key-456",
		"headers": map[string]any{
			"X-Api-Key": "nested-key",
			"User-Agent": "Mozilla",
		},
		"safe_string": "Authorization: public",
	}
	out := tools.MaskSensitiveParams(in)
	if out["Authorization"] != "***" {
		t.Errorf("Authorization not masked: %v", out["Authorization"])
	}
	if out["cookie"] != "***" {
		t.Errorf("cookie not masked: %v", out["cookie"])
	}
	if out["api_key"] != "***" {
		t.Errorf("api_key not masked: %v", out["api_key"])
	}
	if out["url"] != "http://example.com" {
		t.Errorf("url was masked by mistake: %v", out["url"])
	}
	nested := out["headers"].(map[string]any)
	if nested["X-Api-Key"] != "***" {
		t.Errorf("nested X-Api-Key not masked: %v", nested["X-Api-Key"])
	}
	if nested["User-Agent"] != "Mozilla" {
		t.Errorf("User-Agent was masked: %v", nested["User-Agent"])
	}
	// The string-typed Authorization-like value should be masked (looks like header)
	if out["safe_string"] == "Authorization: public" {
		t.Errorf("safe_string (header-shaped) should be masked: %v", out["safe_string"])
	}
}

func TestExecuteTool_RequiresApprovalAndApproves(t *testing.T) {
	coll := &approvalCollector{}
	loop := &AgentLoop{objective: "x"}
	loop.SetProgressCallback(coll.callback)
	sess, cancel := newTestSession(1)
	defer cancel()
	loop.AttachSession(sess)

	// Register a fake high-risk tool that just echoes its params.
	registry := tools.NewToolRegistry()
	registry.Register(stubTool{
		name: "shell",
		exec: func(_ context.Context, p map[string]any) (string, error) {
			return "shell output for: " + p["cmd"].(string), nil
		},
	})
	loop.registry = registry

	// executeTool creates its own PendingApproval; resolve the one that
	// ends up on the session once it's set.
	go func() {
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if p := sess.PendingApproval(); p != nil {
				p.Resolve(ApprovalDecision{ActionID: p.ActionID, Approve: true, Reason: "test"})
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	out, err := loop.executeTool(context.Background(), "shell", `{"cmd":"ls"}`)
	if err != nil {
		t.Fatalf("executeTool error: %v", err)
	}
	if !strings.Contains(out, "shell output for: ls") {
		t.Fatalf("output missing: %q", out)
	}

	// Verify the approval_required event was emitted with action_id and
	// masked params.
	found := false
	for _, ev := range coll.snapshot() {
		if ev.Type == "approval_required" {
			found = true
			if ev.ActionID == "" {
				t.Errorf("approval_required missing action_id")
			}
			if ev.ToolName != "shell" {
				t.Errorf("approval_required tool = %q, want shell", ev.ToolName)
			}
		}
	}
	if !found {
		t.Errorf("no approval_required event captured; events=%+v", coll.snapshot())
	}
}

func TestExecuteTool_DenyReturnsDenyMessage(t *testing.T) {
	loop := &AgentLoop{objective: "x"}
	sess, cancel := newTestSession(1)
	defer cancel()
	loop.AttachSession(sess)

	registry := tools.NewToolRegistry()
	called := false
	registry.Register(stubTool{
		name: "shell",
		exec: func(_ context.Context, p map[string]any) (string, error) {
			called = true
			return "should not run", nil
		},
	})
	loop.registry = registry

	go func() {
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			if p := sess.PendingApproval(); p != nil {
				p.Resolve(ApprovalDecision{ActionID: p.ActionID, Approve: false, Reason: "too aggressive"})
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	out, err := loop.executeTool(context.Background(), "shell", `{"cmd":"rm -rf /"}`)
	if err != nil {
		t.Fatalf("executeTool error: %v", err)
	}
	if !strings.Contains(out, "[TOOL DENIED]") {
		t.Fatalf("expected deny message, got: %q", out)
	}
	if !strings.Contains(out, "too aggressive") {
		t.Fatalf("reason not surfaced: %q", out)
	}
	if called {
		t.Errorf("tool was executed despite deny")
	}
}

func TestExecuteTool_LowRiskNoApproval(t *testing.T) {
	loop := &AgentLoop{objective: "x"}
	sess, cancel := newTestSession(1)
	defer cancel()
	loop.AttachSession(sess)

	registry := tools.NewToolRegistry()
	registry.Register(stubTool{
		name: "http",
		exec: func(_ context.Context, p map[string]any) (string, error) {
			return "http 200 OK", nil
		},
	})
	loop.registry = registry

	out, err := loop.executeTool(context.Background(), "http", `{"url":"http://example.com"}`)
	if err != nil {
		t.Fatalf("executeTool error: %v", err)
	}
	if out != "http 200 OK" {
		t.Fatalf("output = %q, want http 200 OK", out)
	}
	if sess.PendingApproval() != nil {
		t.Errorf("low-risk tool should not have set a pending approval")
	}
}

// stubTool is a test helper for the tools.Tool interface.
type stubTool struct {
	name string
	exec func(context.Context, map[string]any) (string, error)
}

func (s stubTool) Name() string             { return s.name }
func (s stubTool) Description() string      { return "stub " + s.name }
func (s stubTool) Schema() map[string]any   { return map[string]any{"type": "object"} }
func (s stubTool) Execute(ctx context.Context, p map[string]any) (string, error) {
	return s.exec(ctx, p)
}
