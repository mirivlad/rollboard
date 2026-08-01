package game

import "context"

// MiniGameReference pins a launch action to an immutable mini-game module.
// It is intentionally data-only: author definitions never carry executable
// code into the Rollboard process.
type MiniGameReference struct {
	ModuleID string         `json:"moduleId"`
	Version  int            `json:"version"`
	Input    map[string]any `json:"input,omitempty"`
}

// MiniGameModule describes the immutable contract a future sandboxed runner
// will expose. Input and result schemas are declarative JSON schemas owned by
// the module registry, not author-supplied code.
type MiniGameModule struct {
	ID           string         `json:"id"`
	Version      int            `json:"version"`
	Title        string         `json:"title"`
	InputSchema  map[string]any `json:"inputSchema"`
	ResultSchema map[string]any `json:"resultSchema"`
}

// MiniGameInvocation contains only the explicit, server-selected input for a
// mini-game. A runner must not access room storage or mutate a session itself.
type MiniGameInvocation struct {
	Reference MiniGameReference `json:"reference"`
	RoomID    string            `json:"roomId"`
	SessionID string            `json:"sessionId"`
	PlayerID  string            `json:"playerId"`
}

// MiniGameResult is returned to the authoritative server for validation and
// application in the same room transaction in a future release.
type MiniGameResult struct {
	Output map[string]any `json:"output"`
}

// MiniGameRunner is the future isolation boundary. This release does not wire
// one into the runtime; launch_minigame definitions are rejected at validation.
type MiniGameRunner interface {
	Run(context.Context, MiniGameInvocation) (MiniGameResult, error)
}
