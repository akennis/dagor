// Package builtin provides general-purpose operators shipped with the dagor framework.
package builtin

import (
	"context"
	"fmt"
	"log"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/operator"
)

// ---------------------------------------------------------------------------
// CoalesceOp[T] — generic 2-input coalesce
// ---------------------------------------------------------------------------

// CoalesceOp merges two mutually-exclusive conditional branches that each
// produce a value of the same type. It returns the first non-nil input as
// Result.
//
// Use it together with merge: "coalesce" on the vertex so that the engine does
// not propagate skips from the branch that did not run:
//
//	"output": {
//	    "op":    "CoalesceStringOp",   // or CoalesceIntOp, CoalesceFloat64Op, …
//	    "merge": "coalesce",
//	    "inputs":  {"A": "det_result", "B": "ai_result"},
//	    "outputs": {"Result": "time_result"}
//	}
//
// Inputs A and B are both optional pointer fields; the operator errors only
// when every input is nil (i.e. every upstream branch was skipped).
//
// All IOperator methods are implemented by hand so that no codegen is needed
// for generic instantiations.
type CoalesceOp[T any] struct {
	A      *T
	B      *T
	Result T
}

func (op *CoalesceOp[T]) Setup(_ *config.Params) error { return nil }
func (op *CoalesceOp[T]) Reset() error                 { return nil }

func (op *CoalesceOp[T]) Run(_ context.Context) error {
	if op.A != nil {
		op.Result = *op.A
		return nil
	}
	if op.B != nil {
		op.Result = *op.B
		return nil
	}
	return fmt.Errorf("CoalesceOp: all inputs are nil")
}

func (op *CoalesceOp[T]) InputFields() map[string]any {
	return map[string]any{"A": &op.A, "B": &op.B}
}

func (op *CoalesceOp[T]) OutputFields() map[string]any {
	return map[string]any{"Result": &op.Result}
}

func (op *CoalesceOp[T]) SetInputField(field string, value any) error {
	// value may be nil when a coalesce vertex receives no output from a
	// skipped branch; the field retains its zero (nil pointer) value.
	if value == nil {
		return nil
	}
	switch field {
	case "A":
		val, ok := value.(*T)
		if !ok {
			return fmt.Errorf("CoalesceOp field A: expected %T, got %T", (*T)(nil), value)
		}
		op.A = val
	case "B":
		val, ok := value.(*T)
		if !ok {
			return fmt.Errorf("CoalesceOp field B: expected %T, got %T", (*T)(nil), value)
		}
		op.B = val
	default:
		return fmt.Errorf("CoalesceOp: unknown field %q", field)
	}
	return nil
}

func (op *CoalesceOp[T]) ResetFields() {
	op.A = nil
	op.B = nil
	var zero T
	op.Result = zero
}

// ---------------------------------------------------------------------------
// CoalesceNOp[T] — generic N-input coalesce (variable arity)
// ---------------------------------------------------------------------------

// CoalesceNOp is the variable-arity counterpart of CoalesceOp.  The number of
// inputs is configured via the "n" param (must be >= 2).  Input field names
// are Input0, Input1, …, Input(n-1).
//
//	"output": {
//	    "op":     "CoalesceNStringOp",
//	    "merge":  "coalesce",
//	    "params": {"n": 3},
//	    "inputs":  {"Input0": "det_result", "Input1": "ai_result", "Input2": "fb_result"},
//	    "outputs": {"Result": "time_result"}
//	}
type CoalesceNOp[T any] struct {
	n      int
	inputs []*T
	result T
}

func (op *CoalesceNOp[T]) Setup(params *config.Params) error {
	if !params.Exists("n") {
		return fmt.Errorf("CoalesceNOp: params.n is required (must be an integer >= 2)")
	}
	n := params.GetInt("n", 0)
	if n < 2 {
		if s := params.GetString("n", ""); s != "" {
			return fmt.Errorf("CoalesceNOp: params.n must be an integer >= 2, got string %q", s)
		}
		return fmt.Errorf("CoalesceNOp: params.n must be an integer >= 2, got %d", n)
	}
	op.n = n
	op.inputs = make([]*T, n)
	return nil
}

func (op *CoalesceNOp[T]) Reset() error {
	op.n = 0
	op.inputs = nil
	return nil
}

func (op *CoalesceNOp[T]) Run(_ context.Context) error {
	for _, v := range op.inputs {
		if v != nil {
			op.result = *v
			return nil
		}
	}
	return fmt.Errorf("CoalesceNOp: all %d inputs are nil", op.n)
}

func (op *CoalesceNOp[T]) InputFields() map[string]any {
	m := make(map[string]any, op.n)
	for i := range op.n {
		m[fmt.Sprintf("Input%d", i)] = &op.inputs[i]
	}
	return m
}

func (op *CoalesceNOp[T]) OutputFields() map[string]any {
	return map[string]any{"Result": &op.result}
}

func (op *CoalesceNOp[T]) SetInputField(field string, value any) error {
	var idx int
	if _, err := fmt.Sscanf(field, "Input%d", &idx); err != nil {
		return fmt.Errorf("CoalesceNOp: unknown field %q", field)
	}
	if idx < 0 || idx >= op.n {
		return fmt.Errorf("CoalesceNOp: field index %d out of range [0, %d)", idx, op.n)
	}
	if value == nil {
		op.inputs[idx] = nil
		return nil
	}
	val, ok := value.(*T)
	if !ok {
		return fmt.Errorf("CoalesceNOp field %s: expected *%T, got %T", field, (*T)(nil), value)
	}
	op.inputs[idx] = val
	return nil
}

func (op *CoalesceNOp[T]) ResetFields() {
	for i := range op.inputs {
		op.inputs[i] = nil
	}
	var zero T
	op.result = zero
}

// ---------------------------------------------------------------------------
// Pre-registered concrete instantiations
// ---------------------------------------------------------------------------
//
// Import this package with a blank identifier to make these ops available:
//
//	import _ "github.com/wwz16/dagor/operator/builtin"

func init() {
	type entry struct {
		name    string
		factory func() operator.IOperator
	}
	entries := []entry{
		// 2-input coalesce
		{"CoalesceStringOp", func() operator.IOperator { return &CoalesceOp[string]{} }},
		{"CoalesceIntOp", func() operator.IOperator { return &CoalesceOp[int]{} }},
		{"CoalesceFloat64Op", func() operator.IOperator { return &CoalesceOp[float64]{} }},
		{"CoalesceBoolOp", func() operator.IOperator { return &CoalesceOp[bool]{} }},
		// N-input coalesce
		{"CoalesceNStringOp", func() operator.IOperator { return &CoalesceNOp[string]{} }},
		{"CoalesceNIntOp", func() operator.IOperator { return &CoalesceNOp[int]{} }},
		{"CoalesceNFloat64Op", func() operator.IOperator { return &CoalesceNOp[float64]{} }},
		{"CoalesceNBoolOp", func() operator.IOperator { return &CoalesceNOp[bool]{} }},
	}
	for _, e := range entries {
		if err := operator.RegisterOpFactory(e.name, e.factory); err != nil {
			log.Fatalf("builtin: register %s: %v", e.name, err)
		}
	}
}
