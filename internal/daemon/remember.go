package daemon

import "github.com/leefowlercu/agentic-memorizer/internal/registry"

const (
	RememberStatusAdded   = "added"
	RememberStatusUpdated = "updated"
	ForgetStatusForgotten = "forgotten"
)

// RememberRequest defines the payload for /remember.
type RememberRequest struct {
	Path  string                    `json:"path"`
	Patch *registry.PathConfigPatch `json:"patch,omitempty"`
}

// RememberResponse defines the response for /remember.
type RememberResponse struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

// ForgetRequest defines the payload for /forget.
type ForgetRequest struct {
	Path     string `json:"path"`
	KeepData bool   `json:"keep_data"`
}

// ForgetResponse defines the response for /forget.
type ForgetResponse struct {
	Status   string `json:"status"`
	Path     string `json:"path"`
	KeepData bool   `json:"keep_data"`
}
