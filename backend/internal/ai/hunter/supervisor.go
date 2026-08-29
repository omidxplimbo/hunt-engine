package hunter

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter/skills"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/llmclient"
)

// Supervisor orchestrates parallel worker agents, each hunting a different
// bug class. It fans out work based on the objective, collects findings via
// the MessageBus, and aggregates everything into a single HunterResult.
type Supervisor struct {
	llmCfg      *llmclient.Config
	target      string
	objective   string
	bugClasses  []string
	skillLoader *skills.SkillLoader
	bus         *MessageBus
	evidence    *EvidenceStore
	learning    *LearningEngine
	persister   *EvidencePersister
	progressFn  func(AgentEvent)
	session     *HuntSession // optional steering target (T2/T3 wire this in)

	maxWorkers    int
	workerTimeout time.Duration

	mu       sync.RWMutex
	workers  []*AgentLoop // populated by Run, consumed by the steering dispatcher
	dispatchCancel context.CancelFunc
}

// SetPersister enables DB persistence of worker evidence
func (s *Supervisor) SetPersister(p *EvidencePersister) { s.persister = p }

// SetProgress wires a live progress callback (already bound to a target ID)
func (s *Supervisor) SetProgress(fn func(AgentEvent)) { s.progressFn = fn }

// AttachSession binds the supervisor (and all its workers) to a HuntSession
// for mid-run steering. T1 plumbs the option; T3 wires the fan-out.
func (s *Supervisor) AttachSession(sess *HuntSession) { s.session = sess }

// emit publishes an event on both the bus and the progress callback
func (s *Supervisor) emit(eventType, detail, bugClass string) {
	s.bus.Publish("progress", AgentMessage{
		From: "supervisor", Type: eventType, BugClass: bugClass, Payload: detail,
	})
	if s.progressFn != nil {
		s.progressFn(AgentEvent{
			Type:      eventType,
			Detail:    detail,
			BugClass:  bugClass,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// SupervisorResult aggregates results from all workers
type SupervisorResult struct {
	WorkerStatuses []WorkerStatus  `json:"worker_statuses"`
	HunterResults  []*HunterResult `json:"hunter_results"`
	Summary        string          `json:"summary"`
	VulnsFound     int             `json:"vulns_found"`
}

// NewSupervisor creates a supervisor for the given objective.
// If bugClasses is empty, defaults to a standard spread.
func NewSupervisor(
	llmCfg *llmclient.Config,
	target string,
	objective string,
	bugClasses []string,
	skillLoader *skills.SkillLoader,
	evidence *EvidenceStore,
	learning *LearningEngine,
) *Supervisor {
	if len(bugClasses) == 0 {
		bugClasses = []string{"xss", "sqli", "ssrf"}
	}
	return &Supervisor{
		llmCfg:        llmCfg,
		target:        target,
		objective:     objective,
		bugClasses:    bugClasses,
		skillLoader:   skillLoader,
		bus:           NewMessageBus(),
		evidence:      evidence,
		learning:      learning,
		maxWorkers:    3,
		workerTimeout: 10 * time.Minute,
	}
}

// Run executes all workers in parallel (capped at maxWorkers concurrent),
// waits for completion, and returns aggregated results.
func (s *Supervisor) Run(ctx context.Context) (*SupervisorResult, error) {
	log.Printf("[Supervisor] Starting %d workers on %s: %v",
		len(s.bugClasses), s.target, s.bugClasses)

	// T3: launch a steering dispatcher that fans SteerCh commands out to
	// every running worker. Stops when ctx is cancelled or all workers exit.
	dispatchCtx, dispatchCancel := context.WithCancel(ctx)
	defer dispatchCancel()
	s.dispatchCancel = dispatchCancel
	go s.steerDispatcher(dispatchCtx)

	sem := make(chan struct{}, s.maxWorkers)
	var wg sync.WaitGroup
	results := make([]*HunterResult, len(s.bugClasses))
	errs := make([]error, len(s.bugClasses))

	for i, bugClass := range s.bugClasses {
		wg.Add(1)
		go func(idx int, bc string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			name := fmt.Sprintf("worker_%s", bc)
			s.bus.SetWorkerStatus(name, &WorkerStatus{
				Name:      name,
				BugClass:  bc,
				Status:    "running",
				StartedAt: time.Now(),
			})
			s.emit("worker_start", fmt.Sprintf("Worker %s started", name), bc)

			wctx, cancel := context.WithTimeout(ctx, s.workerTimeout)
			defer cancel()

			result, err := s.runWorker(wctx, bc)
			if err != nil {
				log.Printf("[Supervisor] Worker %s failed: %v", name, err)
				now := time.Now()
				s.bus.SetWorkerStatus(name, &WorkerStatus{
					Name: name, BugClass: bc, Status: "failed",
					StartedAt: now, FinishedAt: now, Error: err.Error(),
				})
				s.emit("worker_failed", fmt.Sprintf("Worker %s failed: %v", name, err), bc)
				errs[idx] = err
				return
			}

			results[idx] = result
			finished := time.Now()
			startedAt := finished.Add(-s.workerElapsed(result))
			s.bus.SetWorkerStatus(name, &WorkerStatus{
				Name: name, BugClass: bc, Status: "completed",
				StartedAt: startedAt, FinishedAt: finished,
			})
			s.emit("worker_done", fmt.Sprintf("Worker %s completed: %s", name, result.Summary), bc)
		}(i, bugClass)
	}

	wg.Wait()

	totalVulns := 0
	for _, r := range results {
		if r != nil {
			totalVulns += r.VulnsFound
		}
	}

	summary := fmt.Sprintf(
		"Supervisor completed multi-agent hunt on %s. Objective: %s. "+
			"Dispatched %d workers (%v). Found %d vulnerabilities in total.",
		s.target, s.objective, len(s.bugClasses), s.bugClasses, totalVulns)

	return &SupervisorResult{
		WorkerStatuses: s.bus.GetWorkerStatus(),
		HunterResults:  nonNil(results),
		Summary:        summary,
		VulnsFound:     totalVulns,
	}, nil
}

// steerDispatcher consumes SteerCh and broadcasts commands to every
// worker loop. Used in multi mode; single-mode AgentLoop drains its own
// channel directly. Stops when ctx is done (i.e. supervisor Run returns
// or the session is cancelled).
func (s *Supervisor) steerDispatcher(ctx context.Context) {
	if s.session == nil {
		return
	}
	ch := s.session.SteerCh
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-ch:
			if !ok {
				return
			}
			s.broadcastSteer(cmd)
		}
	}
}

// broadcastSteer fans a single SteerCommand out to every currently
// registered worker. Workers added after the broadcast won't see it —
// acceptable because new workers pick up session state on the next turn.
func (s *Supervisor) broadcastSteer(cmd SteerCommand) {
	s.mu.RLock()
	workers := make([]*AgentLoop, len(s.workers))
	copy(workers, s.workers)
	s.mu.RUnlock()

	for _, w := range workers {
		switch cmd.Type {
		case SteerMessage:
			w.EnqueueMessage(cmd.Content)
		case SteerSetObjective:
			w.SetObjective(cmd.Content)
		case SteerPause:
			w.RequestPause()
		case SteerResume:
			w.Resume()
		case SteerCancel:
			w.Cancel()
		}
	}
	s.emit("steer_broadcast", string(cmd.Type), "")
}

// runWorker runs a single specialized agent loop scoped to one bug class.
// The loop is registered with the supervisor so the steering dispatcher
// (T3) can broadcast commands to it.
func (s *Supervisor) runWorker(ctx context.Context, bugClass string) (*HunterResult, error) {
	objective := fmt.Sprintf("%s Focus specifically on %s vulnerabilities.", s.objective, bugClass)
	loop := NewAgentLoop(s.llmCfg, s.target, objective, s.skillLoader, s.evidence, s.learning)
	if s.persister != nil {
		workerPersister := NewEvidencePersister(s.persister.db, s.persister.targetID,
			s.persister.userID, s.persister.ownerKey, "worker_"+bugClass)
		loop.SetPersister(workerPersister)
	}
	if s.progressFn != nil {
		bc := bugClass
		loop.SetProgressCallback(func(ev AgentEvent) {
			ev.BugClass = bc
			s.progressFn(ev)
		})
	}
	// Each worker also gets its own session channel slice so it could
	// drain commands independently in the future. For now the supervisor
	// dispatcher is the only reader; the loop drains nothing.
	if s.session != nil {
		loop.AttachSession(s.session)
	}

	// Register the worker for the dispatcher's broadcast map. Removed on
	// exit so we don't leak references after the worker finishes.
	s.mu.Lock()
	s.workers = append(s.workers, loop)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		for i, w := range s.workers {
			if w == loop {
				s.workers = append(s.workers[:i], s.workers[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
	}()

	return loop.Run(ctx)
}

func (s *Supervisor) workerElapsed(r *HunterResult) time.Duration {
	if r == nil || r.Strategy == nil {
		return 0
	}
	return 0 // statuses are approximate; precise timing kept simple
}

func nonNil(rs []*HunterResult) []*HunterResult {
	out := make([]*HunterResult, 0, len(rs))
	for _, r := range rs {
		if r != nil {
			out = append(out, r)
		}
	}
	return out
}
