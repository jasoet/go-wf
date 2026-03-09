package function

import (
	"context"
	"fmt"
	"sync"
)

// FunctionInput is the input passed to registered handler functions.
type FunctionInput struct {
	Args    map[string]string `json:"args,omitempty"`
	Data    []byte            `json:"data,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
}

// FunctionOutput is the output returned by registered handler functions.
type FunctionOutput struct {
	Result map[string]string `json:"result,omitempty"`
	Data   []byte            `json:"data,omitempty"`
}

// Handler is the function signature for registered handlers.
type Handler func(ctx context.Context, input FunctionInput) (*FunctionOutput, error)

// Registry maps function names to handlers.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewRegistry creates a new empty function registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register adds a named handler to the registry.
func (r *Registry) Register(name string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

// Get retrieves a handler by name.
func (r *Registry) Get(name string) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("function %q not found in registry", name)
	}
	return h, nil
}

// Has returns true if a handler with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.handlers[name]
	return ok
}
