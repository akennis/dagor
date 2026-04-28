package predicate

import (
	"fmt"
	"sync"
)

// Predicate evaluates a node's input fields and returns true if the node should execute.
type Predicate func(inputs map[string]any) bool

var globalRegistry = &registry{m: sync.Map{}}

type registry struct{ m sync.Map }

// Register registers a named predicate. Returns error if already taken.
func Register(name string, pred Predicate) error {
	if name == "" {
		return fmt.Errorf("predicate name is required")
	}
	if pred == nil {
		return fmt.Errorf("predicate is required")
	}
	if _, loaded := globalRegistry.m.LoadOrStore(name, pred); loaded {
		return fmt.Errorf("predicate already registered: %s", name)
	}
	return nil
}

// Unregister removes a named predicate. No-op if not registered.
func Unregister(name string) {
	globalRegistry.m.Delete(name)
}

// MustReplace registers or replaces a named predicate unconditionally.
// Idempotent: safe to call in test setup where the predicate may already be registered.
func MustReplace(name string, pred Predicate) {
	globalRegistry.m.Store(name, pred)
}

// Get retrieves a predicate by name.
func Get(name string) (Predicate, error) {
	p, ok := globalRegistry.m.Load(name)
	if !ok {
		return nil, fmt.Errorf("predicate not found: %s", name)
	}
	return p.(Predicate), nil
}
