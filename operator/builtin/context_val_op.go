package builtin

import (
	"context"
	"fmt"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/operator"
)

// ContextValOp extracts a value of type T from the run context at execution time.
// The context key is fixed at registration; each eng.Run(ctx) call can supply a
// different value by storing it under that key with context.WithValue.
//
// Use ContextValFactory to build the factory and register it once at startup:
//
//	type myKey struct{}
//	operator.RegisterOpFactory("MyInputOp", builtin.ContextValFactory[float64](myKey{}))
//
//	ctx = context.WithValue(ctx, myKey{}, 3.14)
//	eng.Run(ctx)
type ContextValOp[T any] struct {
	Result T
	key    any
}

func (op *ContextValOp[T]) Setup(_ *config.Params) error { return nil }
func (op *ContextValOp[T]) Reset() error                 { return nil }

func (op *ContextValOp[T]) Run(ctx context.Context) error {
	v, ok := ctx.Value(op.key).(T)
	if !ok {
		var zero T
		return fmt.Errorf("ContextValOp: context key %v not found or has wrong type (want %T)", op.key, zero)
	}
	op.Result = v
	return nil
}

func (op *ContextValOp[T]) InputFields() map[string]any  { return map[string]any{} }
func (op *ContextValOp[T]) OutputFields() map[string]any { return map[string]any{"Result": &op.Result} }

func (op *ContextValOp[T]) SetInputField(field string, _ any) error {
	return fmt.Errorf("ContextValOp: no input fields (got %q)", field)
}

func (op *ContextValOp[T]) ResetFields() {
	var zero T
	op.Result = zero
}

// ContextValFactory returns a factory for operator.RegisterOpFactory.
// It creates a ContextValOp[T] that reads key from ctx on every Run call.
func ContextValFactory[T any](key any) func() operator.IOperator {
	return func() operator.IOperator { return &ContextValOp[T]{key: key} }
}
