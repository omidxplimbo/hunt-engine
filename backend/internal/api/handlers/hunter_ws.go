package handlers

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter"
)

// huntProgressHub fans out live AgentEvents to all WebSocket subscribers
// watching a target's hunt.
type huntProgressHub struct {
	mu   sync.Mutex
	subs map[uint]map[*websocket.Conn]chan []byte
	last map[uint][]hunter.AgentEvent // replay buffer (last 100 events per target)
}

var progressHub = &huntProgressHub{
	subs: make(map[uint]map[*websocket.Conn]chan []byte),
	last: make(map[uint][]hunter.AgentEvent),
}

// PublishHuntEvent broadcasts an agent event to subscribers of the target.
// Called by HunterAgent while a hunt runs.
func PublishHuntEvent(targetID uint, ev hunter.AgentEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	progressHub.mu.Lock()
	buf := progressHub.last[targetID]
	buf = append(buf, ev)
	if len(buf) > 100 {
		buf = buf[len(buf)-100:]
	}
	progressHub.last[targetID] = buf
	targetSubs := make(map[*websocket.Conn]chan []byte, len(progressHub.subs[targetID]))
	for c, ch := range progressHub.subs[targetID] {
		targetSubs[c] = ch
	}
	progressHub.mu.Unlock()

	for _, ch := range targetSubs {
		select {
		case ch <- data:
		default:
		}
	}
}

// wsClientMessage is the wire schema the operator sends over the WS.
// Only one of Content/ActionID/Reason/Objective is populated per message,
// depending on Type.
type wsClientMessage struct {
	Type      string `json:"type"` // message | set_objective | pause | resume | cancel | approve | deny | ping
	Content   string `json:"content,omitempty"`
	ActionID  string `json:"action_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Objective string `json:"objective,omitempty"`
}

// HuntProgressWS upgrades to WebSocket, streams live hunt events
// (server -> client), and accepts steer + approval commands
// (client -> server). The function expects the auth middleware to have
// already validated the token and set user context.
//
// Concurrency: a single writer goroutine consumes from `out` and writes
// to the connection; a single reader goroutine reads from the connection
// and routes commands to the session. gorilla/websocket terminates on
// any ReadMessage error, so the reader exits on the first error and the
// writer observes the closed `out` channel and returns. No retries.
func HuntProgressWS(c *websocket.Conn) {
	targetID, err := parseUint(c.Params("id"))
	if err != nil {
		c.Close()
		return
	}

	// Extract user context set by Protected() middleware
	if _, ok := c.Locals("user_id").(uint); !ok {
		if _, ok := c.Locals("user_id").(float64); !ok {
			c.Close()
			return
		}
	}

	// Single outbound channel: agent events + protocol responses (pong,
	// errors). One writer goroutine drains it.
	out := make(chan []byte, 128)
	progressHub.mu.Lock()
	if progressHub.subs[targetID] == nil {
		progressHub.subs[targetID] = make(map[*websocket.Conn]chan []byte)
	}
	progressHub.subs[targetID][c] = out

	// Replay recent events so late joiners get context
	replay := make([]hunter.AgentEvent, len(progressHub.last[targetID]))
	copy(replay, progressHub.last[targetID])
	progressHub.mu.Unlock()

	// Find the active session for this target (if any). The session is
	// looked up on every command so a hunt that starts after the WS
	// connects still works.
	sess := HuntSessions.FirstActive(targetID)

	// Write replay events into out so the writer goroutine picks them up.
	for _, ev := range replay {
		if data, err := json.Marshal(ev); err == nil {
			out <- data
		}
	}

	// readErr signals the reader goroutine terminating on error.
	readErr := make(chan error, 1)
	done := make(chan struct{})

	// Reader goroutine: parses client messages, routes them to the
	// session's SteerCh / pending approval. Exits on first read error.
	go func() {
		defer close(done)
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				readErr <- err
				return
			}
			handleClientMessage(targetID, msg, &sess, out)
		}
	}()

	// Writer goroutine: drains out, writes to the WS. Exits when out
	// closes (deferred below).
	go func() {
		pingTicker := time.NewTicker(30 * time.Second)
		defer pingTicker.Stop()
		for {
			select {
			case <-done:
				return
			case data, ok := <-out:
				if !ok {
					return
				}
				if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
					return
				}
			case <-pingTicker.C:
				// Best-effort ping; ignored on error.
				_ = c.WriteMessage(websocket.PingMessage, nil)
			}
		}
	}()

	defer func() {
		progressHub.mu.Lock()
		if subs := progressHub.subs[targetID]; subs != nil {
			delete(subs, c)
			if len(subs) == 0 {
				delete(progressHub.subs, targetID)
			}
		}
		progressHub.mu.Unlock()
		close(out)
		c.Close()
	}()

	// Block until the reader exits. Any read error (including normal
	// client close) takes us down here.
	if err := <-readErr; err != nil {
		log.Printf("[HunterWS] read loop ended for target %d: %v", targetID, err)
	}
}

// handleClientMessage parses one operator command and dispatches it to
// the session. Unknown types and parse errors produce an error frame on
// `out`; they do NOT close the connection (the spec says "server
// rejects unknown type with an error response and does NOT kill the
// connection" — applies to the message, not the connection itself).
// However, gorilla/websocket's ReadMessage on a malformed text frame
// can still fail at the protocol layer; the per-connection error budget
// is enforced by the read goroutine exiting on the first error.
func handleClientMessage(targetID uint, raw []byte, sessRef **hunter.HuntSession, out chan<- []byte) {
	var msg wsClientMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		sendError(out, "invalid json", "")
		return
	}

	// Refresh session pointer every command so a hunt started after the
	// WS connection was opened still works.
	if *sessRef == nil {
		*sessRef = HuntSessions.FirstActive(targetID)
	}
	sess := *sessRef

	switch msg.Type {
	case "ping":
		sendPong(out)
	case "message":
		if sess == nil {
			sendError(out, "no active session for this target", msg.Type)
			return
		}
		sess.EnqueueSteer(hunter.SteerCommand{Type: hunter.SteerMessage, Content: msg.Content})
	case "set_objective":
		if sess == nil {
			sendError(out, "no active session for this target", msg.Type)
			return
		}
		content := msg.Content
		if content == "" {
			content = msg.Objective
		}
		sess.EnqueueSteer(hunter.SteerCommand{Type: hunter.SteerSetObjective, Content: content})
	case "pause":
		if sess == nil {
			sendError(out, "no active session for this target", msg.Type)
			return
		}
		sess.EnqueueSteer(hunter.SteerCommand{Type: hunter.SteerPause})
	case "resume":
		if sess == nil {
			sendError(out, "no active session for this target", msg.Type)
			return
		}
		sess.EnqueueSteer(hunter.SteerCommand{Type: hunter.SteerResume})
	case "cancel":
		if sess == nil {
			sendError(out, "no active session for this target", msg.Type)
			return
		}
		sess.EnqueueSteer(hunter.SteerCommand{Type: hunter.SteerCancel})
	case "approve":
		if sess == nil {
			sendError(out, "no active session for this target", msg.Type)
			return
		}
		if p := sess.PendingApproval(); p != nil {
			p.Resolve(hunter.ApprovalDecision{ActionID: msg.ActionID, Approve: true})
		} else {
			sendError(out, "no pending approval to resolve", msg.Type)
		}
	case "deny":
		if sess == nil {
			sendError(out, "no active session for this target", msg.Type)
			return
		}
		if p := sess.PendingApproval(); p != nil {
			p.Resolve(hunter.ApprovalDecision{ActionID: msg.ActionID, Approve: false, Reason: msg.Reason})
		} else {
			sendError(out, "no pending approval to resolve", msg.Type)
		}
	case "operator_answer":
		// T14: the agent asked the operator a question via the
		// ask_operator tool. Deliver the answer to the matching
		// pending OperatorChannel entry (matched by action_id).
		if sess == nil {
			sendError(out, "no active session for this target", msg.Type)
			return
		}
		if msg.ActionID == "" {
			sendError(out, "operator_answer: action_id is required", msg.Type)
			return
		}
		if sess.Operator() == nil {
			sendError(out, "no operator channel on this session", msg.Type)
			return
		}
		if !sess.Operator().Resolve(msg.ActionID, msg.Content) {
			sendError(out, "operator_answer: no pending question with that action_id (timed out or already answered)", msg.Type)
			return
		}
		// Confirm to the operator that their answer was delivered.
		ev := hunter.AgentEvent{
			Type:      "operator_accepted",
			Detail:    "answer delivered to agent",
			ActionID:  msg.ActionID,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		if data, err := json.Marshal(ev); err == nil {
			select {
			case out <- data:
			default:
			}
		}
	default:
		sendError(out, "unknown command type", msg.Type)
	}
}

func sendError(out chan<- []byte, detail, cmdType string) {
	ev := hunter.AgentEvent{
		Type:      "error",
		Detail:    detail,
		BugClass:  cmdType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if data, err := json.Marshal(ev); err == nil {
		select {
		case out <- data:
		default:
		}
	}
}

func sendPong(out chan<- []byte) {
	ev := hunter.AgentEvent{
		Type:      "pong",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if data, err := json.Marshal(ev); err == nil {
		select {
		case out <- data:
		default:
		}
	}
}

func parseUint(s string) (uint, error) {
	var n uint64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fiber.NewError(fiber.StatusBadRequest, "invalid id")
		}
		n = n*10 + uint64(r-'0')
	}
	return uint(n), nil
}

func init() {
	log.SetFlags(log.LstdFlags)
}
