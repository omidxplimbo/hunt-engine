package handlers

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter"
)

// captureOut drains `out` for a bounded time and returns every frame.
func captureOut(out <-chan []byte, d time.Duration) [][]byte {
	var got [][]byte
	deadline := time.After(d)
	for {
		select {
		case data, ok := <-out:
			if !ok {
				return got
			}
			got = append(got, data)
		case <-deadline:
			return got
		}
	}
}

func mustEvent(t *testing.T, data []byte) hunter.AgentEvent {
	t.Helper()
	var ev hunter.AgentEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("unmarshal: %v (raw: %s)", err, data)
	}
	return ev
}

func newTestWSHandlerDeps() (*hunter.HuntSession, *hunter.SessionStore, chan []byte) {
	store := hunter.NewSessionStore()
	sess := hunter.NewHuntSession(7, 1, "test-owner", "single", "find xss")
	store.Add(sess)
	out := make(chan []byte, 32)
	return sess, store, out
}

func TestHandleClientMessage_Ping(t *testing.T) {
	sess, _, out := newTestWSHandlerDeps()
	defer HuntSessionsCleanup()

	var sessRef = sess
	handleClientMessage(7, []byte(`{"type":"ping"}`), &sessRef, out)

	frames := captureOut(out, 50*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if ev := mustEvent(t, frames[0]); ev.Type != "pong" {
		t.Errorf("frame type = %q, want pong", ev.Type)
	}
}

func TestHandleClientMessage_MessageRoutesToSteerCh(t *testing.T) {
	_, store, out := newTestWSHandlerDeps()
	defer HuntSessionsCleanup()
	// Replace HuntSessions global so the message handler finds the session.
	prev := HuntSessions
	HuntSessions = store
	defer func() { HuntSessions = prev }()

	var sessRef *hunter.HuntSession
	handleClientMessage(7, []byte(`{"type":"message","content":"focus on /search"}`), &sessRef, out)

	sess := store.FirstActive(7)
	if sess == nil {
		t.Fatalf("session not in store")
	}
	select {
	case cmd := <-sess.SteerCh:
		if cmd.Type != hunter.SteerMessage || cmd.Content != "focus on /search" {
			t.Errorf("steer command = %+v", cmd)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("steer command not delivered")
	}
}

func TestHandleClientMessage_PauseResumeCancel(t *testing.T) {
	_, store, out := newTestWSHandlerDeps()
	defer HuntSessionsCleanup()
	prev := HuntSessions
	HuntSessions = store
	defer func() { HuntSessions = prev }()

	var sessRef *hunter.HuntSession
	handleClientMessage(7, []byte(`{"type":"pause"}`), &sessRef, out)
	handleClientMessage(7, []byte(`{"type":"resume"}`), &sessRef, out)
	handleClientMessage(7, []byte(`{"type":"cancel"}`), &sessRef, out)

	sess := store.FirstActive(7)
	if sess == nil {
		t.Fatalf("session not in store")
	}
	// Drain and verify each command arrived in order.
	want := []hunter.SteerCommandType{
		hunter.SteerPause,
		hunter.SteerResume,
		hunter.SteerCancel,
	}
	for _, w := range want {
		select {
		case cmd := <-sess.SteerCh:
			if cmd.Type != w {
				t.Errorf("got %v, want %v", cmd.Type, w)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("missing steer command %v", w)
		}
	}
}

func TestHandleClientMessage_ApproveResolvesPending(t *testing.T) {
	_, store, out := newTestWSHandlerDeps()
	defer HuntSessionsCleanup()
	prev := HuntSessions
	HuntSessions = store
	defer func() { HuntSessions = prev }()

	sess := store.FirstActive(7)
	approval := hunter.NewPendingApprovalForTest("shell", map[string]any{"cmd": "ls"})
	sess.SetPendingApproval(approval)

	var sessRef = sess
	handleClientMessage(7, []byte(`{"type":"approve","action_id":"`+approval.ActionID+`"}`), &sessRef, out)

	select {
	case d := <-approval.Decision():
		if !d.Approve {
			t.Errorf("decision not approved")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("decision not delivered")
	}
}

func TestHandleClientMessage_DenyWithReason(t *testing.T) {
	_, store, out := newTestWSHandlerDeps()
	defer HuntSessionsCleanup()
	prev := HuntSessions
	HuntSessions = store
	defer func() { HuntSessions = prev }()

	sess := store.FirstActive(7)
	approval := hunter.NewPendingApprovalForTest("shell", map[string]any{"cmd": "rm -rf /"})
	sess.SetPendingApproval(approval)

	var sessRef = sess
	handleClientMessage(7, []byte(`{"type":"deny","action_id":"`+approval.ActionID+`","reason":"too aggressive"}`), &sessRef, out)

	select {
	case d := <-approval.Decision():
		if d.Approve {
			t.Errorf("decision not denied")
		}
		if d.Reason != "too aggressive" {
			t.Errorf("reason = %q", d.Reason)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("decision not delivered")
	}
}

func TestHandleClientMessage_UnknownTypeEmitsError(t *testing.T) {
	_, store, out := newTestWSHandlerDeps()
	defer HuntSessionsCleanup()
	prev := HuntSessions
	HuntSessions = store
	defer func() { HuntSessions = prev }()

	var sessRef *hunter.HuntSession
	handleClientMessage(7, []byte(`{"type":"explode"}`), &sessRef, out)

	frames := captureOut(out, 50*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("expected 1 error frame, got %d", len(frames))
	}
	if ev := mustEvent(t, frames[0]); ev.Type != "error" {
		t.Errorf("frame type = %q, want error", ev.Type)
	}
}

func TestHandleClientMessage_NoSessionEmitsError(t *testing.T) {
	store := hunter.NewSessionStore() // empty
	prev := HuntSessions
	HuntSessions = store
	defer func() { HuntSessions = prev }()
	defer HuntSessionsCleanup()

	out := make(chan []byte, 32)
	var sessRef *hunter.HuntSession
	handleClientMessage(99, []byte(`{"type":"message","content":"x"}`), &sessRef, out)

	frames := captureOut(out, 50*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("expected 1 error frame, got %d", len(frames))
	}
	if ev := mustEvent(t, frames[0]); ev.Type != "error" {
		t.Errorf("frame type = %q, want error", ev.Type)
	}
}

func TestHandleClientMessage_InvalidJSON(t *testing.T) {
	_, store, out := newTestWSHandlerDeps()
	defer HuntSessionsCleanup()
	prev := HuntSessions
	HuntSessions = store
	defer func() { HuntSessions = prev }()

	var sessRef *hunter.HuntSession
	handleClientMessage(7, []byte(`not json`), &sessRef, out)

	frames := captureOut(out, 50*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("expected 1 error frame, got %d", len(frames))
	}
	if ev := mustEvent(t, frames[0]); ev.Type != "error" {
		t.Errorf("frame type = %q, want error", ev.Type)
	}
}

// HuntSessionsCleanup is a best-effort helper for tests: we don't
// actually need to clear anything because HuntSessions is replaced
// per-test, but having a defer target keeps test bodies readable.
func HuntSessionsCleanup() {}

// Keep a sync import marker so the test file compiles regardless of
// future changes.
var _ = sync.Mutex{}
