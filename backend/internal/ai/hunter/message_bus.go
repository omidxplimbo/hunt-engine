package hunter

import (
	"sync"
	"time"
)

// AgentMessage is a message exchanged between supervisor and workers
type AgentMessage struct {
	From      string    `json:"from"`
	Type      string    `json:"type"` // finding, progress, error, request_task, task_result
	BugClass  string    `json:"bug_class,omitempty"`
	Payload   string    `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

// WorkerStatus tracks the state of a worker agent
type WorkerStatus struct {
	Name       string    `json:"name"`
	BugClass   string    `json:"bug_class"`
	Status     string    `json:"status"` // pending, running, completed, failed, cancelled
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// MessageBus coordinates communication between supervisor and workers
type MessageBus struct {
	mu        sync.Mutex
	subscribers map[string][]chan AgentMessage
	findings  []AgentMessage
	statuses  map[string]*WorkerStatus
	closed    bool
}

// NewMessageBus creates a new message bus
func NewMessageBus() *MessageBus {
	return &MessageBus{
		subscribers: make(map[string][]chan AgentMessage),
		statuses:    make(map[string]*WorkerStatus),
	}
}

// Subscribe registers a channel for messages of a given topic
func (b *MessageBus) Subscribe(topic string) chan AgentMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan AgentMessage, 64)
	b.subscribers[topic] = append(b.subscribers[topic], ch)
	return ch
}

// Publish sends a message to all subscribers of a topic
func (b *MessageBus) Publish(topic string, msg AgentMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	msg.Timestamp = time.Now()
	if msg.Type == "finding" {
		b.findings = append(b.findings, msg)
	}
	for _, ch := range b.subscribers[topic] {
		select {
		case ch <- msg:
		default: // don't block on slow consumers
		}
	}
}

// SetWorkerStatus records a worker's status transition
func (b *MessageBus) SetWorkerStatus(name string, ws *WorkerStatus) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.statuses[name] = ws
}

// GetWorkerStatus returns a copy of all worker statuses
func (b *MessageBus) GetWorkerStatus() []WorkerStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]WorkerStatus, 0, len(b.statuses))
	for _, ws := range b.statuses {
		out = append(out, *ws)
	}
	return out
}

// GetFindings returns all findings published so far
func (b *MessageBus) GetFindings() []AgentMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]AgentMessage, len(b.findings))
	copy(out, b.findings)
	return out
}

// Close shuts down the bus and drains subscriber channels
func (b *MessageBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, chans := range b.subscribers {
		for _, ch := range chans {
			close(ch)
		}
	}
}
