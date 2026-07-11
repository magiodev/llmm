package backend

import (
	"fmt"
)

// Backend defines the interface for LLM runtime backends
type Backend interface {
	// Name returns the backend name (e.g., "turboquant", "ds4")
	Name() string

	// ModelKeys returns available model keys for this backend
	ModelKeys() []string

	// Start starts the backend service
	Start() error

	// Stop stops the backend service
	Stop() error

	// Restart restarts the backend service
	Restart() error

	// Status returns the current status of the backend
	Status() string

	// SwitchModel switches to the specified model key
	SwitchModel(key string) error

	// ServiceName returns the systemd user service name
	ServiceName() string

	// Port returns the port this backend runs on
	Port() int
}

// Default backends
var Backends = map[string]func() Backend{
	"turboquant": func() Backend { return &TurboQuant{} },
	"ds4":        func() Backend { return &DS4{} },
	"vllm":       func() Backend { return &VLLM{} },
	"comfyui":    func() Backend { return &ComfyUI{} },
}

// GetBackend returns a backend instance by name
func GetBackend(name string) (Backend, error) {
	factory, ok := Backends[name]
	if !ok {
		return nil, fmt.Errorf("unknown backend: %s", name)
	}
	return factory(), nil
}
