package builtin

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/operator"
)

// ---------------------------------------------------------------------------
// ConstOp[T] — generic constant-value emitter
// ---------------------------------------------------------------------------

// ConstOp emits a constant scalar value of type T configured via a "Value"
// string param parsed at Setup time. It has no inputs and one output ("Result").
//
// Use the registered factory names (ConstStringOp, ConstIntOp, etc.) rather
// than instantiating ConstOp[T] directly — the parse function is only set by
// the factory.
//
//	"const_pi": {
//	    "op":     "ConstFloat64Op",
//	    "params": {"Value": "3.14159"},
//	    "outputs": {"Result": "pi"}
//	}
//
// All IOperator methods are implemented by hand so that no codegen is needed
// for generic instantiations.
type ConstOp[T any] struct {
	Result T
	value  T
	parse  func(string) (T, error)
}

func (op *ConstOp[T]) Setup(params *config.Params) error {
	if op.parse == nil {
		return fmt.Errorf("ConstOp: parse function not set (use the registered factory)")
	}
	s := params.GetString("Value", "")
	v, err := op.parse(s)
	if err != nil {
		return fmt.Errorf("ConstOp: invalid Value %q: %w", s, err)
	}
	op.value = v
	return nil
}

func (op *ConstOp[T]) Reset() error {
	var zero T
	op.value = zero
	return nil
}

func (op *ConstOp[T]) Run(_ context.Context) error {
	op.Result = op.value
	return nil
}

func (op *ConstOp[T]) InputFields() map[string]any {
	return map[string]any{}
}

func (op *ConstOp[T]) OutputFields() map[string]any {
	return map[string]any{"Result": &op.Result}
}

func (op *ConstOp[T]) SetInputField(field string, _ any) error {
	return fmt.Errorf("ConstOp: no input fields (got %q)", field)
}

func (op *ConstOp[T]) ResetFields() {
	var zero T
	op.Result = zero
}

// ---------------------------------------------------------------------------
// Pre-registered concrete instantiations
// ---------------------------------------------------------------------------
//
// Import this package with a blank identifier to make these ops available:
//
//	import _ "github.com/akennis/dagor/operator/builtin"

func newConstFactory[T any](parse func(string) (T, error)) func() operator.IOperator {
	return func() operator.IOperator {
		return &ConstOp[T]{parse: parse}
	}
}

func init() {
	type entry struct {
		name    string
		factory func() operator.IOperator
	}
	entries := []entry{
		{"ConstStringOp", newConstFactory(func(s string) (string, error) { return s, nil })},
		{"ConstIntOp", newConstFactory(func(s string) (int, error) { return strconv.Atoi(s) })},
		{"ConstFloat64Op", newConstFactory(func(s string) (float64, error) { return strconv.ParseFloat(s, 64) })},
		{"ConstBoolOp", newConstFactory(func(s string) (bool, error) { return strconv.ParseBool(s) })},
	}
	for _, e := range entries {
		if err := operator.RegisterOpFactory(e.name, e.factory); err != nil {
			log.Fatalf("builtin: register %s: %v", e.name, err)
		}
	}
}
