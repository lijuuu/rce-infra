package dto

// RunCommand represents a command execution request
type RunCommand struct {
	Cmd        string   `json:"cmd" validate:"required"`
	Args       []string `json:"args,omitempty"`
	TimeoutSec int      `json:"timeout_sec,omitempty"`
}

// UpdateAgent represents an agent update request
type UpdateAgent struct {
	Version string `json:"version" validate:"required"`
	URL     string `json:"url" validate:"required,url"`
}

// UpdatePackage represents a package update request
type UpdatePackage struct {
	Packages []string `json:"packages" validate:"required"`
	Action   string   `json:"action" validate:"required,oneof=install remove upgrade"`
}

// CommandRegistry maps command types to their struct types for validation
var CommandRegistry = map[string]interface{}{
	"RunCommand":    RunCommand{},
	"UpdateAgent":   UpdateAgent{},
	"UpdatePackage": UpdatePackage{},
}
