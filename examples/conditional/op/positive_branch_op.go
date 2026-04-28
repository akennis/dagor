package op

import (
	"context"
	"fmt"
	"log"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/operator"
)

// PositiveBranchOp is the "deterministic / fast-path" operator used in the coalesce
// demo.  It multiplies its integer input by 10, simulating a cheap lookup.
type PositiveBranchOp struct {
	in  *int `dag:"input"`
	out int  `dag:"output"`
}

func (op *PositiveBranchOp) Setup(_ *config.Params) error { return nil }
func (op *PositiveBranchOp) Reset() error                 { return nil }

func (op *PositiveBranchOp) Run(_ context.Context) error {
	if op.in == nil {
		return fmt.Errorf("DetBranchOp: missing required input")
	}
	op.out = *op.in * 10
	return nil
}

func (op *PositiveBranchOp) InputFields() map[string]any  { return map[string]any{"in": &op.in} }
func (op *PositiveBranchOp) OutputFields() map[string]any { return map[string]any{"out": &op.out} }

func (op *PositiveBranchOp) SetInputField(field string, value any) error {
	if value == nil {
		return nil
	}
	if field != "in" {
		return fmt.Errorf("DetBranchOp: unknown field %q", field)
	}
	val, ok := value.(*int)
	if !ok {
		return fmt.Errorf("DetBranchOp: field in expected *int, got %T", value)
	}
	op.in = val
	return nil
}

func (op *PositiveBranchOp) ResetFields() {
	op.in = nil
	op.out = 0
}

func init() {
	if err := operator.RegisterOp[PositiveBranchOp](); err != nil {
		log.Fatalf("RegisterOp[DetBranchOp] error: %v", err)
	}
}
