package hunter

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OperatorQuestionTimeout is how long ask_operator blocks waiting for
// the operator's answer before auto-skipping. 5 minutes is the longest
// acceptable pause for a security test — beyond that, the LLM should
// pick a different path.
const OperatorQuestionTimeout = 5 * time.Minute

// operatorSkipMarker is the string the LLM sees if the operator does
// not answer within OperatorQuestionTimeout. It is structured so the
// LLM can pattern-match on "[OPERATOR DID NOT ANSWER" and reroute.
const operatorSkipMarker = "[OPERATOR DID NOT ANSWER WITHIN 5 MINUTES — skipping]"

// operatorQuestion is one in-flight operator question. The actionID
// is a uuid; the channel carries the answer once the operator responds.
type operatorQuestion struct {
	actionID  string
	question  string
	askedAt   time.Time
	answerCh  chan string
}

// OperatorChannel is the per-session conduit between the AgentLoop
// (which blocks on ask) and the WS read goroutine / HTTP handler
// (which dispatches the operator's answer).
//
// Concurrency: the channel is protected by a sync.Mutex. The AgentLoop
// calls Ask (which adds an entry and blocks on its answerCh); the WS
// read goroutine calls Resolve (which finds the entry and writes the
// answer). Both can be in flight simultaneously for the same session.
type OperatorChannel struct {
	mu       sync.Mutex
	pending  map[string]*operatorQuestion
}

// NewOperatorChannel creates an empty channel.
func NewOperatorChannel() *OperatorChannel {
	return &OperatorChannel{pending: make(map[string]*operatorQuestion)}
}

// Ask blocks until the operator answers the question or the context is
// cancelled / the per-question timeout elapses. Returns the answer
// string on success, the operatorSkipMarker on timeout, or ctx.Err()
// on context cancellation.
func (c *OperatorChannel) Ask(ctx context.Context, question string) (string, error) {
	q := &operatorQuestion{
		actionID: uuid.NewString(),
		question: question,
		askedAt:  time.Now().UTC(),
		answerCh: make(chan string, 1),
	}
	c.mu.Lock()
	c.pending[q.actionID] = q
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, q.actionID)
		c.mu.Unlock()
	}()

	timer := time.NewTimer(OperatorQuestionTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
		return operatorSkipMarker, nil
	case answer, ok := <-q.answerCh:
		if !ok {
			return operatorSkipMarker, nil
		}
		return answer, nil
	}
}

// ActionID is exposed for tests and the WS handler. It is set on the
// returned question by the Ask caller; we expose it via a helper
// that returns the actionID of the most-recent Ask so the AgentLoop
// can include it in the operator_question event.
func (c *OperatorChannel) lastActionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var last string
	var lastTime time.Time
	for id, q := range c.pending {
		if q.askedAt.After(lastTime) {
			lastTime = q.askedAt
			last = id
		}
	}
	return last
}

// Resolve delivers an answer to the question with the given actionID.
// No-op if the question is not pending (e.g. it already timed out).
// Returns true if the answer was delivered.
func (c *OperatorChannel) Resolve(actionID, answer string) bool {
	c.mu.Lock()
	q, ok := c.pending[actionID]
	c.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case q.answerCh <- answer:
		return true
	default:
		return false
	}
}

// CancelAll signals every pending question with the skip marker. Used
// when the session is cancelled so blocked AgentLoops unblock.
func (c *OperatorChannel) CancelAll() {
	c.mu.Lock()
	pending := make([]*operatorQuestion, 0, len(c.pending))
	for _, q := range c.pending {
		pending = append(pending, q)
	}
	c.mu.Unlock()
	for _, q := range pending {
		select {
		case q.answerCh <- operatorSkipMarker:
		default:
		}
	}
}

// Pending returns a snapshot of every currently-pending question.
// Used by the GET /operator-questions API and the WS reader.
func (c *OperatorChannel) Pending() []PendingOperatorQuestion {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]PendingOperatorQuestion, 0, len(c.pending))
	for _, q := range c.pending {
		out = append(out, PendingOperatorQuestion{
			ActionID: q.actionID,
			Question: q.question,
			AskedAt:  q.askedAt,
		})
	}
	return out
}

// PendingOperatorQuestion is the JSON-serializable view of a pending
// question. Returned by GET /operator-questions and emitted as the
// payload of the operator_question WS event.
type PendingOperatorQuestion struct {
	ActionID string    `json:"action_id"`
	Question string    `json:"question"`
	AskedAt  time.Time `json:"asked_at"`
}

// ErrOperatorChannelClosed is returned by Ask if the channel has been
// closed (e.g. after a session cancellation that preempted the answer).
var ErrOperatorChannelClosed = errors.New("operator channel closed")
