package handlers

import (
	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter"
)

// HuntSessions is the in-process registry of active hunt sessions.
// Initialized once at server startup (see cmd/server/main.go).
var HuntSessions = hunter.NewSessionStore()
