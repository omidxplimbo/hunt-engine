package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeOperatorChannel is a test double for OperatorChannel. It blocks
// on Ask until the test calls deliver, which writes the answer to all
// pending askers.
type fakeOperatorChannel struct {
	deliver chan string
	pending []chan string
}

func (f *fakeOperatorChannel) Ask(ctx context.Context, question string) (string, error) {
	ch := make(chan string, 1)
	f.pending = append(f.pending, ch)
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case ans := <-ch:
		return ans, nil
	}
}

func (f *fakeOperatorChannel) resolveAll(answer string) {
	for _, ch := range f.pending {
		select {
		case ch <- answer:
		default:
		}
	}
}

func TestAskOperatorTool_Name(t *testing.T) {
	a := NewAskOperatorTool()
	if a.Name() != "ask_operator" {
		t.Errorf("Name() = %q, want ask_operator", a.Name())
	}
}

func TestAskOperatorTool_Schema_HasQuestion(t *testing.T) {
	a := NewAskOperatorTool()
	schema := a.Schema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no properties: %+v", schema)
	}
	if _, ok := props["question"]; !ok {
		t.Errorf("schema missing 'question' field: %+v", props)
	}
}

func TestAskOperatorTool_Execute_MissingQuestion(t *testing.T) {
	a := NewAskOperatorTool()
	_, err := a.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatalf("Execute should error when question is missing")
	}
	if !strings.Contains(err.Error(), "question") {
		t.Errorf("error should mention question: %v", err)
	}
}

func TestAskOperatorTool_Execute_NoChannel(t *testing.T) {
	a := NewAskOperatorTool()
	_, err := a.Execute(context.Background(), map[string]any{"question": "x"})
	if err == nil {
		t.Fatalf("Execute should error when no channel is in context")
	}
	if !strings.Contains(err.Error(), "operator channel") {
		t.Errorf("error should mention operator channel: %v", err)
	}
}

func TestAskOperatorTool_Execute_ResolvesAnswer(t *testing.T) {
	a := NewAskOperatorTool()
	fake := &fakeOperatorChannel{}

	ctx := WithOperatorChannel(context.Background(), fake)
	// Resolve in a goroutine after Ask is parked.
	go func() {
		// Give the Ask goroutine a moment to park on the channel.
		// (We can't time this exactly, but a few ms is enough.)
		// The fake records the channel in pending.
		for i := 0; i < 100; i++ {
			if len(fake.pending) == 1 {
				break
			}
			// busy-wait briefly
			// nolint
		}
		fake.resolveAll("yes go ahead")
	}()

	answer, err := a.Execute(ctx, map[string]any{"question": "test /admin?"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if answer != "yes go ahead" {
		t.Errorf("answer = %q, want 'yes go ahead'", answer)
	}
}

func TestAskOperatorTool_Execute_ContextCancel(t *testing.T) {
	a := NewAskOperatorTool()
	fake := &fakeOperatorChannel{}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = WithOperatorChannel(ctx, fake)

	go func() {
		// Cancel after a brief delay so the test doesn't hang.
		// nolint
		cancel()
	}()

	_, err := a.Execute(ctx, map[string]any{"question": "test?"})
	if err == nil {
		t.Fatalf("Execute should error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should be context.Canceled, got: %v", err)
	}
}
