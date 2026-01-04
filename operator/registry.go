package operator

import (
	"fmt"
	"reflect"
	"sync"
)

var (
	operatorRegistry = NewOperatorRegistry()
)

// OperatorRegistry is a factory for creating operators.
// Each operator has a operator pool.
type OperatorRegistry struct {
	opPools sync.Map // name -> operator pool
}

// NewOperatorRegistry creates a new operator registry.
func NewOperatorRegistry() *OperatorRegistry {
	return &OperatorRegistry{
		opPools: sync.Map{},
	}
}

func (f *OperatorRegistry) Register(name string, opBuilder func() IOperator) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if opBuilder == nil {
		return fmt.Errorf("opBuilder is required")
	}
	if _, ok := f.opPools.Load(name); ok {
		return fmt.Errorf("operator pool already registered for name: %s", name)
	}
	pool := NewOperatorPool(name, opBuilder)
	f.opPools.Store(name, pool)
	return nil
}

func (f *OperatorRegistry) Get(name string) (IOperator, error) {
	pool, ok := f.opPools.Load(name)
	if !ok {
		return nil, fmt.Errorf("operator pool not found for name: %s", name)
	}
	return pool.(*OperatorPool).Get(), nil
}

func (f *OperatorRegistry) Put(name string, op IOperator) error {
	pool, ok := f.opPools.Load(name)
	if !ok {
		return fmt.Errorf("operator pool not found for name: %s", name)
	}
	pool.(*OperatorPool).Put(op)
	return nil
}

// RegisterOp registers a new operator.
// It should be called in init function of the operator package.
func RegisterOp[T any]() error {
	var instance any = new(T) // 此时 instance 是 *T
	if _, ok := instance.(IOperator); !ok {
		return fmt.Errorf("type %T must implement IOperator", instance)
	}

	t := reflect.TypeOf(instance).Elem()
	opName := t.Name()
	if opName == "" {
		return fmt.Errorf("operator name is empty")
	}

	// register operator to registry
	return operatorRegistry.Register(opName, func() IOperator {
		// create new operator instance
		return reflect.New(t).Interface().(IOperator)
	})
}

// GetOp gets the operator by name.
func GetOp(name string) (IOperator, error) {
	return operatorRegistry.Get(name)
}

// PutOp puts the operator back to the pool.
// It should be called after the operator is used.
func PutOp(name string, op IOperator) error {
	return operatorRegistry.Put(name, op)
}
