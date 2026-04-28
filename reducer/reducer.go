package reducer

import (
	"fmt"
	"sync"
)

// Reducer folds a running accumulator and the current element into a new accumulator.
type Reducer func(acc any, item any) any

var globalRegistry = &registry{m: sync.Map{}}

type registry struct{ m sync.Map }

// Register registers a named reducer. Returns an error if the name is already taken.
func Register(name string, r Reducer) error {
	if name == "" {
		return fmt.Errorf("reducer name is required")
	}
	if r == nil {
		return fmt.Errorf("reducer is required")
	}
	if _, loaded := globalRegistry.m.LoadOrStore(name, r); loaded {
		return fmt.Errorf("reducer already registered: %s", name)
	}
	return nil
}

// Unregister removes a named reducer. No-op if not registered.
func Unregister(name string) {
	globalRegistry.m.Delete(name)
}

// MustReplace registers or replaces a named reducer unconditionally.
// Idempotent: safe to call in test setup where the reducer may already be registered.
func MustReplace(name string, r Reducer) {
	globalRegistry.m.Store(name, r)
}

// Get retrieves a reducer by name.
func Get(name string) (Reducer, error) {
	r, ok := globalRegistry.m.Load(name)
	if !ok {
		return nil, fmt.Errorf("reducer not found: %s", name)
	}
	return r.(Reducer), nil
}
