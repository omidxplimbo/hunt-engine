package hunter

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SteerCommandType is the kind of human steering action queued for a session.
type SteerCommandType string

const (
	SteerMessage      SteerCommandType = "message"
	SteerSetObjective SteerCommandType = "set_objective"
	SteerPause        SteerCommandType = "pause"
	SteerResume       SteerCommandType = "resume"
	SteerCancel       SteerCommandType = "cancel"
)

// SteerCommand is a single human steering action. Sent over SteerCh and
// consumed by the agent loop between turns.
type SteerCommand struct {
	Type    SteerCommandType `json:"type"`
	Content string           `json:"content,omitempty"`
}

// ApprovalDecision is the operator's response to an approval_required event.
type ApprovalDecision struct {
	ActionID string `json:"action_id"`
	Approve  bool   `json:"approve"`
	Reason   string `json:"reason,omitempty"`
}

// PendingApproval describes a tool call waiting for human approval.
// Loop fills it before emitting the approval_required event, then blocks
// on ApproveCh (or timeout).
type PendingApproval struct {
	ActionID  string         `json:"action_id"`
	Tool      string         `json:"tool"`
	Params    map[string]any `json:"params"`
	Requested time.Time      `json:"requested_at"`
	mu        sync.Mutex
	resolved  bool
	decision  chan ApprovalDecision
}

// newPendingApproval creates a PendingApproval with a fresh uuid. Used
// internally by AgentLoop and exported as NewPendingApprovalForTest for
// handler tests that need to seed an approval without going through the
// full agent loop.
func newPendingApproval(tool string, params map[string]any) *PendingApproval {
	return &PendingApproval{
		ActionID:  uuid.NewString(),
		Tool:      tool,
		Params:    params,
		Requested: time.Now(),
		decision:  make(chan ApprovalDecision, 1),
	}
}

// NewPendingApprovalForTest exposes newPendingApproval to the handler
// test package. Kept separate from newPendingApproval so the test
// surface doesn't leak into the production hunter package.
func NewPendingApprovalForTest(tool string, params map[string]any) *PendingApproval {
	return newPendingApproval(tool, params)
}

// Decision returns the channel the loop blocks on. Resolved once and only once.
func (p *PendingApproval) Decision() <-chan ApprovalDecision {
	return p.decision
}

// Resolve records the operator's decision. Idempotent — second call is a no-op.
func (p *PendingApproval) Resolve(d ApprovalDecision) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.resolved {
		return
	}
	p.resolved = true
	p.decision <- d
	close(p.decision)
}

// HuntSession is one running (or recently finished) hunt for a target.
// SteerCh, ApproveCh, and CancelFn are the contract with the agent loop
// (and the WS read goroutine that feeds them).
type HuntSession struct {
	ID         string    `json:"id"`
	TargetID   uint      `json:"target_id"`
	UserID     uint      `json:"user_id"`
	OwnerKey   string    `json:"-"`
	Mode       string    `json:"mode"`
	Objective  string    `json:"objective"`
	Status     string    `json:"status"` // running | paused | cancelled | completed | failed
	StartedAt  time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	SteerCh  chan SteerCommand  // buffered, drained by the loop between turns
	CancelFn context.CancelFunc // detaches the loop from the request ctx

	mu              sync.RWMutex
	paused          bool
	pendingApproval *PendingApproval
	operator        *OperatorChannel  // T14: per-session conduit for ask_operator
	lastError       string
	summary         string
	vulnsFound      int

	// Loop is set after the goroutine starts so the WS handler can read
	// pending approval / live status. Nil until then.
	loop *AgentLoop
}

// NewHuntSession creates a fresh session and its channels. The session is
// NOT yet registered — callers (handler) must call SessionStore.Add.
func NewHuntSession(targetID, userID uint, ownerKey, mode, objective string) *HuntSession {
	return &HuntSession{
		ID:        uuid.NewString(),
		TargetID:  targetID,
		UserID:    userID,
		OwnerKey:  ownerKey,
		Mode:      mode,
		Objective: objective,
		Status:    "running",
		StartedAt: time.Now().UTC(),
		SteerCh:   make(chan SteerCommand, 32),
		operator:  NewOperatorChannel(),
	}
}

// PauseRequested returns true if a pause was queued. Caller (loop) flips
// its internal paused flag and emits the event.
func (s *HuntSession) PauseRequested() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	was := s.paused
	s.paused = true
	return !was
}

// ResumeRequested clears the paused flag.
func (s *HuntSession) ResumeRequested() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	was := s.paused
	s.paused = false
	return was
}

// IsPaused reports the current pause state.
func (s *HuntSession) IsPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.paused
}

// SetPendingApproval stores the in-flight approval so the WS handler can
// resolve it when the operator clicks Approve/Deny.
func (s *HuntSession) SetPendingApproval(p *PendingApproval) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingApproval = p
}

// ClearPendingApproval is called after the operator responds so the next
// tool call can re-arm.
func (s *HuntSession) ClearPendingApproval() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingApproval = nil
}

// PendingApproval returns the current pending approval, or nil.
func (s *HuntSession) PendingApproval() *PendingApproval {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pendingApproval
}

// MarkStatus updates the session status and (for terminal states) records
// the finished time.
func (s *HuntSession) MarkStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Status == "cancelled" || s.Status == "completed" || s.Status == "failed" {
		return
	}
	s.Status = status
	if status == "completed" || status == "cancelled" || status == "failed" {
		now := time.Now().UTC()
		s.FinishedAt = &now
	}
}

// SetResult stores the final summary and vuln count for the session listing.
func (s *HuntSession) SetResult(summary string, vulns int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summary = summary
	s.vulnsFound = vulns
}

// SetError stores the last error (for failed status).
func (s *HuntSession) SetError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = msg
}

// Snapshot is a read-only view for the HTTP listing endpoint.
type SessionSnapshot struct {
	ID         string     `json:"id"`
	TargetID   uint       `json:"target_id"`
	UserID     uint       `json:"user_id"`
	Mode       string     `json:"mode"`
	Objective  string     `json:"objective"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Paused     bool       `json:"paused"`
	Summary    string     `json:"summary,omitempty"`
	VulnsFound int        `json:"vulns_found,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
}

func (s *HuntSession) Snapshot() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SessionSnapshot{
		ID:         s.ID,
		TargetID:   s.TargetID,
		UserID:     s.UserID,
		Mode:       s.Mode,
		Objective:  s.Objective,
		Status:     s.Status,
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
		Paused:     s.paused,
		Summary:    s.summary,
		VulnsFound: s.vulnsFound,
		LastError:  s.lastError,
	}
}

// SetLoop records the agent loop for status / approval lookups.
func (s *HuntSession) SetLoop(l *AgentLoop) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loop = l
}

// Loop returns the agent loop, or nil if it hasn't started yet.
func (s *HuntSession) GetLoop() *AgentLoop {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loop
}

// Operator returns the per-session OperatorChannel used by the
// ask_operator tool (T14). Returns nil only for sessions created
// before T14 or for tests that bypass NewHuntSession.
func (s *HuntSession) Operator() *OperatorChannel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.operator
}

// EnqueueSteer pushes a command into the session's channel. Non-blocking;
// if the channel is full the command is dropped and the caller logs.
func (s *HuntSession) EnqueueSteer(cmd SteerCommand) bool {
	select {
	case s.SteerCh <- cmd:
		return true
	default:
		return false
	}
}

// SessionStore is the in-process registry of active sessions. One per
// backend process; not persisted across restarts (intentional, see spec S3).
type SessionStore struct {
	mu       sync.RWMutex
	byTarget map[uint]map[string]*HuntSession // targetID -> sessionID -> session
}

// NewSessionStore creates an empty store.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		byTarget: make(map[uint]map[string]*HuntSession),
	}
}

// Add registers a session under its target.
func (s *SessionStore) Add(sess *HuntSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byTarget[sess.TargetID] == nil {
		s.byTarget[sess.TargetID] = make(map[string]*HuntSession)
	}
	s.byTarget[sess.TargetID][sess.ID] = sess
}

// Remove drops a session from the registry. Safe to call twice.
func (s *SessionStore) Remove(targetID uint, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.byTarget[targetID]; m != nil {
		delete(m, sessionID)
		if len(m) == 0 {
			delete(s.byTarget, targetID)
		}
	}
}

// Get returns the session, or nil.
func (s *SessionStore) Get(targetID uint, sessionID string) *HuntSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m := s.byTarget[targetID]; m != nil {
		return m[sessionID]
	}
	return nil
}

// FirstActive returns the first non-terminal session for a target, or nil.
// Used by the WS handler when a new connection arrives without a sessionID.
func (s *SessionStore) FirstActive(targetID uint) *HuntSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sess := range s.byTarget[targetID] {
		st := sess.Status
		if st == "running" || st == "paused" {
			return sess
		}
	}
	return nil
}

// ListForTarget returns snapshots of all sessions for a target (any status).
// Caller can filter by status client-side.
func (s *SessionStore) ListForTarget(targetID uint) []SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []SessionSnapshot{}
	for _, sess := range s.byTarget[targetID] {
		out = append(out, sess.Snapshot())
	}
	return out
}
