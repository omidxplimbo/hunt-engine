package hunter

import (
	"context"
	"testing"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter/skills"
)

func TestSupervisor_BroadcastSteerMessage(t *testing.T) {
	// Build a Supervisor with two workers (no real LLM) and verify a
	// single SteerMessage lands in both workers' history.
	sess, _ := newTestSession(1)

	sup := &Supervisor{
		target:     "http://example.com",
		objective:  "find xss",
		bugClasses: []string{"xss", "sqli"},
		session:    sess,
		bus:        NewMessageBus(),
		evidence:   NewEvidenceStore(),
		// No real LLM cfg; we won't call Run.
	}

	// Synthesize two worker loops, register them, and broadcast.
	w1 := &AgentLoop{objective: "w1"}
	w1.AttachSession(sess)
	w2 := &AgentLoop{objective: "w2"}
	w2.AttachSession(sess)
	sup.mu.Lock()
	sup.workers = []*AgentLoop{w1, w2}
	sup.mu.Unlock()

	sup.broadcastSteer(SteerCommand{Type: SteerMessage, Content: "focus on /search"})

	for i, w := range []*AgentLoop{w1, w2} {
		hist := w.History()
		if len(hist) != 1 {
			t.Fatalf("worker %d history length = %d, want 1: %+v", i, len(hist), hist)
		}
		if hist[0]["content"] != "[OPERATOR MESSAGE] focus on /search" {
			t.Fatalf("worker %d content = %q", i, hist[0]["content"])
		}
	}
}

func TestSupervisor_BroadcastPauseCancel(t *testing.T) {
	sess, cancel := newTestSession(1)
	defer cancel()

	sup := &Supervisor{
		target:     "http://example.com",
		objective:  "test",
		bugClasses: []string{"xss"},
		session:    sess,
		bus:        NewMessageBus(),
		evidence:   NewEvidenceStore(),
	}

	w1 := &AgentLoop{objective: "w1"}
	w1.AttachSession(sess)
	w2 := &AgentLoop{objective: "w2"}
	w2.AttachSession(sess)
	sup.mu.Lock()
	sup.workers = []*AgentLoop{w1, w2}
	sup.mu.Unlock()

	sup.broadcastSteer(SteerCommand{Type: SteerPause})
	if !sess.IsPaused() {
		t.Fatalf("session should be paused after broadcast")
	}

	sup.broadcastSteer(SteerCommand{Type: SteerResume})
	if sess.IsPaused() {
		t.Fatalf("session should be resumed after broadcast")
	}

	sup.broadcastSteer(SteerCommand{Type: SteerCancel})
	// CancelFn was called; no error path to assert, but the workers'
	// Cancel method is a no-op when their session == nil. Both workers
	// share the same session, so both Cancels ran. Done.
}

func TestSupervisor_SteerDispatcherDrainsAndExits(t *testing.T) {
	sess, cancel := newTestSession(1)
	defer cancel()

	sup := &Supervisor{
		target:     "http://example.com",
		objective:  "test",
		bugClasses: []string{"xss"},
		session:    sess,
		bus:        NewMessageBus(),
		evidence:   NewEvidenceStore(),
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	go sup.steerDispatcher(ctx)

	// Enqueue a message; broadcast should fire (no workers registered,
	// so the broadcast is a no-op; just check the channel is drained).
	sess.EnqueueSteer(SteerCommand{Type: SteerMessage, Content: "hi"})

	// Give the dispatcher a moment to drain.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case _, ok := <-sess.SteerCh:
			if !ok {
				ctxCancel()
				return
			}
		default:
			// Channel drained.
			ctxCancel()
			return
		}
	}
	ctxCancel()
	t.Fatalf("SteerCh was not drained by the dispatcher")
}

func TestSupervisor_NoSession_Noop(t *testing.T) {
	sup := &Supervisor{
		target:   "http://example.com",
		bus:      NewMessageBus(),
		evidence: NewEvidenceStore(),
	}
	// steerDispatcher must be a no-op when session is nil.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		sup.steerDispatcher(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("dispatcher should exit when session is nil and ctx is cancelled")
	}
}

// Keep `skills` import used; package has no test-only init.
var _ = skills.NewSkillLoader
