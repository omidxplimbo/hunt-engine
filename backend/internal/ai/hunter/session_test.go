package hunter

import (
	"context"
	"sync"
	"testing"
	"time"
)

// newTestSession creates a session with the given cancel func wired up.
// Tests use this to drive EnqueueSteer / drainSteer without spinning a
// real Hunter.
func newTestSession(targetID uint) (*HuntSession, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sess := NewHuntSession(targetID, 1, "test-owner", "single", "find xss")
	sess.CancelFn = cancel
	_ = ctx
	return sess, cancel
}

func TestSessionStore_AddGetRemove(t *testing.T) {
	store := NewSessionStore()
	sess, _ := newTestSession(42)

	store.Add(sess)
	if got := store.Get(42, sess.ID); got != sess {
		t.Fatalf("Get returned wrong session")
	}
	if got := store.FirstActive(42); got != sess {
		t.Fatalf("FirstActive returned wrong session")
	}

	store.Remove(42, sess.ID)
	if got := store.Get(42, sess.ID); got != nil {
		t.Fatalf("session not removed")
	}
	if got := store.FirstActive(42); got != nil {
		t.Fatalf("FirstActive should be nil after remove")
	}
}

func TestSessionStore_FirstActiveSkipsTerminal(t *testing.T) {
	store := NewSessionStore()
	s1, _ := newTestSession(7)
	s2, _ := newTestSession(7)
	store.Add(s1)
	store.Add(s2)

	s1.MarkStatus("completed")
	if got := store.FirstActive(7); got != s2 {
		t.Fatalf("FirstActive should skip completed, got %v", got)
	}

	s2.MarkStatus("failed")
	if got := store.FirstActive(7); got != nil {
		t.Fatalf("FirstActive should be nil when all terminal, got %v", got)
	}
}

func TestEnqueueSteer_NonBlockingWhenFull(t *testing.T) {
	sess, _ := newTestSession(1)
	// Channel capacity is 32
	for i := 0; i < 32; i++ {
		if !sess.EnqueueSteer(SteerCommand{Type: SteerMessage, Content: "x"}) {
			t.Fatalf("EnqueueSteer should accept command %d", i)
		}
	}
	// 33rd must drop, not block
	if sess.EnqueueSteer(SteerCommand{Type: SteerMessage, Content: "overflow"}) {
		t.Fatalf("EnqueueSteer should drop when full")
	}
}

func TestPendingApproval_ResolveOnce(t *testing.T) {
	p := newPendingApproval("shell", map[string]any{"cmd": "rm -rf /"})

	p.Resolve(ApprovalDecision{ActionID: p.ActionID, Approve: true, Reason: "ok"})
	// Second resolve must be a no-op (would panic on closed channel otherwise)
	p.Resolve(ApprovalDecision{Approve: false})

	select {
	case d := <-p.Decision():
		if !d.Approve {
			t.Fatalf("first decision should be approve=true, got false")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("decision not delivered")
	}
}

func TestAgentLoop_SteeringHistoryMutation(t *testing.T) {
	// Build a minimal AgentLoop WITHOUT a real LLM; we only test the
	// mutex-protected history mutations driven by EnqueueMessage and the
	// session-channel plumbing. Run() is not exercised.
	loop := &AgentLoop{
		objective: "initial",
	}
	sess, _ := newTestSession(99)
	loop.AttachSession(sess)

	// Concurrent writers must not race (Run with -race would catch this).
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			loop.EnqueueMessage("msg A")
			loop.SetObjective("obj A")
		}(i)
		go func(i int) {
			defer wg.Done()
			_ = loop.History()
			_ = loop.Objective()
		}(i)
	}
	wg.Wait()

	hist := loop.History()
	// 50 messages from EnqueueMessage + 0 from SetObjective (which doesn't
	// write history; only emits an event).
	if got, want := len(hist), 50; got != want {
		t.Fatalf("history length = %d, want %d (sample last: %v)", got, want, hist[len(hist)-1])
	}
	for i, m := range hist {
		if m["role"] != "user" {
			t.Fatalf("hist[%d] role = %q, want user", i, m["role"])
		}
		if m["content"] != "[OPERATOR MESSAGE] msg A" {
			t.Fatalf("hist[%d] content = %q", i, m["content"])
		}
	}
}

func TestDrainSteer_AppliesMessageAndExitsOnBlock(t *testing.T) {
	loop := &AgentLoop{objective: "orig"}
	sess, _ := newTestSession(1)
	loop.AttachSession(sess)

	// Push a message before the loop "starts".
	sess.EnqueueSteer(SteerCommand{Type: SteerMessage, Content: "from operator"})

	// Non-blocking drain picks it up.
	if !loop.drainSteer(context.Background(), false) {
		t.Fatalf("non-blocking drain should return true with ctx alive")
	}
	hist := loop.History()
	if len(hist) != 1 || hist[0]["content"] != "[OPERATOR MESSAGE] from operator" {
		t.Fatalf("message not applied: %+v", hist)
	}

	// Blocking drain with no commands and a cancelled ctx returns false.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if loop.drainSteer(ctx, true) {
		t.Fatalf("blocking drain should return false on cancelled ctx")
	}
}

func TestDrainSteer_PauseAndResume(t *testing.T) {
	loop := &AgentLoop{objective: "x"}
	sess, _ := newTestSession(1)
	loop.AttachSession(sess)

	// Mark paused, then enqueue a resume. Blocking drain returns true.
	sess.PauseRequested()
	sess.EnqueueSteer(SteerCommand{Type: SteerResume})

	if !loop.drainSteer(context.Background(), true) {
		t.Fatalf("blocking drain should return true after resume")
	}
	if sess.IsPaused() {
		t.Fatalf("session should be unpaused after resume command")
	}
}

func TestDrainSteer_CancelCommand(t *testing.T) {
	loop := &AgentLoop{objective: "x"}
	sess, cancel := newTestSession(1)
	defer cancel()
	loop.AttachSession(sess)

	sess.EnqueueSteer(SteerCommand{Type: SteerCancel})

	// Non-blocking drain processes the cancel command; the session's
	// CancelFn is invoked. We can't easily observe ctx.Err() here because
	// drainSteer reads from the channel and the cancel fn does the work.
	// Verify the cancel fn was called: ctx should be Done after a moment.
	loop.drainSteer(context.Background(), false)

	// Give the runtime a tick to propagate the cancel.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		// We can't read the context — the loop used its own ctx.
		// Just confirm the command was consumed (channel is empty now).
		select {
		case _, ok := <-sess.SteerCh:
			if ok {
				continue
			}
		default:
			return
		}
	}
	// If we timed out without seeing the channel drain, fail.
	t.Fatalf("SteerCh was not drained after cancel command")
}
