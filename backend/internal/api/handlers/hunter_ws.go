package handlers

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter"
)

// huntProgressHub fans out live AgentEvents to all WebSocket subscribers
// watching a target's hunt.
type huntProgressHub struct {
	mu    sync.Mutex
	subs  map[uint]map[*websocket.Conn]chan []byte
	last  map[uint][]hunter.AgentEvent // replay buffer (last 100 events per target)
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

// HuntProgressWS upgrades to WebSocket and streams live hunt events.
func HuntProgressWS(c *websocket.Conn) {
	targetID, err := parseUint(c.Params("id"))
	if err != nil {
		c.Close()
		return
	}

	ch := make(chan []byte, 128)
	progressHub.mu.Lock()
	if progressHub.subs[targetID] == nil {
		progressHub.subs[targetID] = make(map[*websocket.Conn]chan []byte)
	}
	progressHub.subs[targetID][c] = ch

	// Replay recent events so late joiners get context
	replay := progressHub.last[targetID]
	progressHub.mu.Unlock()

	for _, ev := range replay {
		if data, err := json.Marshal(ev); err == nil {
			_ = c.WriteMessage(websocket.TextMessage, data)
		}
	}

	defer func() {
		progressHub.mu.Lock()
		if subs := progressHub.subs[targetID]; subs != nil {
			delete(subs, c)
			if len(subs) == 0 {
				delete(progressHub.subs, targetID)
			}
		}
		progressHub.mu.Unlock()
		close(ch)
		c.Close()
	}()

	for {
		msg, ok := <-ch
		if !ok {
			return
		}
		if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
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
