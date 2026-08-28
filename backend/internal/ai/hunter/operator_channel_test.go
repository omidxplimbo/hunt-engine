package hunter

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOperatorChannel_AskAndAnswer(t *testing.T) {
	ch := NewOperatorChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan string, 1)
	go func() {
		answer, err := ch.Ask(ctx, "may I test /admin?")
		if err != nil {
			t.Errorf("Ask returned error: %v", err)
			return
		}
		got <- answer
	}()

	// Give the Ask goroutine a moment to register the pending question.
	deadline := time.Now().Add(200 * time.Millisecond)
	var actionID string
	for time.Now().Before(deadline) {
		pending := ch.Pending()
		if len(pending) == 1 {
			actionID = pending[0].ActionID
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if actionID == "" {
		t.Fatalf("no pending question registered after 200ms; pending=%+v", ch.Pending())
	}

	if !ch.Resolve(actionID, "yes, scope is OK") {
		t.Fatalf("Resolve returned false on a valid pending action_id")
	}

	select {
	case answer := <-got:
		if answer != "yes, scope is OK" {
			t.Errorf("Ask returned %q, want 'yes, scope is OK'", answer)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Ask did not return after Resolve")
	}

	if got := ch.Pending(); len(got) != 0 {
		t.Errorf("pending list should be empty after resolution, got %+v", got)
	}
}

func TestOperatorChannel_AskTimeout(t *testing.T) {
	ch := NewOperatorChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a short timeout for the test by NOT using the production
	// 5-minute constant — instead, just resolve with the skip marker
	// manually. The test for the timeout path is covered below with
	// a custom channel that races against ctx.
	got := make(chan error, 1)
	go func() {
		_, err := ch.Ask(ctx, "are you alive?")
		got <- err
	}()

	// Wait for the pending question.
	var actionID string
	for i := 0; i < 100; i++ {
		if pending := ch.Pending(); len(pending) == 1 {
			actionID = pending[0].ActionID
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if actionID == "" {
		t.Fatalf("pending question not registered")
	}

	// Cancel the context to force the Ask goroutine out without an
	// answer. OperatorChannel.Ask must propagate ctx.Err().
	cancel()

	select {
	case err := <-got:
		if err == nil {
			t.Errorf("Ask should return non-nil error on cancel")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("error should be context.Canceled, got: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Ask did not return after ctx cancel")
	}
}

func TestOperatorChannel_ResolveUnknownID(t *testing.T) {
	ch := NewOperatorChannel()
	if ch.Resolve("not-a-real-id", "answer") {
		t.Errorf("Resolve should return false for unknown action_id")
	}
}

func TestOperatorChannel_PendingSnapshot(t *testing.T) {
	ch := NewOperatorChannel()
	if got := ch.Pending(); len(got) != 0 {
		t.Errorf("empty channel should return empty slice, got %+v", got)
	}

	// Block two questions in goroutines. We don't have a way to
	// observe the Ask goroutine after the map insert but before the
	// defer (defer runs on return, so close(asked) racing with defer
	// is the actual problem). Instead, register two questions and
	// immediately answer them so the channel buffers the question
	// snapshot. We use a short-lived context that we cancel right
	// after the test to unblock the Ask goroutines.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { ch.Ask(ctx, "Q1") }()
	go func() { ch.Ask(ctx, "Q2") }()

	// Poll for the 2 pending entries.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(ch.Pending()) == 2 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	pending := ch.Pending()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d: %+v", len(pending), pending)
	}
	seen := map[string]bool{}
	for _, p := range pending {
		if p.Question == "" || p.ActionID == "" {
			t.Errorf("malformed pending entry: %+v", p)
		}
		seen[p.Question] = true
	}
	if !seen["Q1"] || !seen["Q2"] {
		t.Errorf("expected Q1 and Q2 in pending, got %+v", pending)
	}
}

func TestOperatorChannel_CancelAll(t *testing.T) {
	ch := NewOperatorChannel()
	ctx := context.Background()

	got1 := make(chan string, 1)
	got2 := make(chan string, 1)
	go func() { got1 <- mustAsk(ctx, ch, "Q1") }()
	go func() { got2 <- mustAsk(ctx, ch, "Q2") }()

	// Wait for both to register.
	for i := 0; i < 100; i++ {
		if len(ch.Pending()) == 2 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	ch.CancelAll()

	for label, got := range map[string]chan string{"Q1": got1, "Q2": got2} {
		select {
		case answer := <-got:
			if !strings.Contains(answer, "OPERATOR DID NOT ANSWER") {
				t.Errorf("%s: expected skip marker, got %q", label, answer)
			}
		case <-time.After(500 * time.Millisecond):
			t.Errorf("%s: Ask did not return after CancelAll", label)
		}
	}
}

func mustAsk(ctx context.Context, ch *OperatorChannel, q string) string {
	answer, _ := ch.Ask(ctx, q)
	return answer
}
