package op

import (
	"context"
	"fmt"
	"log"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/operator"
)

// NegativeBranchStep2Op is the second step operator in the negative path used in the coalesce demo.
// It adds 1 to its integer input.
type NegativeBranchStep2Op struct {
	in  *int `dag:"input"`
	out int  `dag:"output"`
}

func (op *NegativeBranchStep2Op) Setup(_ *config.Params) error { return nil }
func (op *NegativeBranchStep2Op) Reset() error                 { return nil }

func (op *NegativeBranchStep2Op) Run(_ context.Context) error {
	if op.in == nil {
		return fmt.Errorf("NegativeStep2BranchOp: missing required input")
	}
	op.out = *op.in + 1
	return nil
}

func (op *NegativeBranchStep2Op) InputFields() map[string]any  { return map[string]any{"in": &op.in} }
func (op *NegativeBranchStep2Op) OutputFields() map[string]any { return map[string]any{"out": &op.out} }

func (op *NegativeBranchStep2Op) SetInputField(field string, value any) error {
	if value == nil {
		return nil
	}
	if field != "in" {
		return fmt.Errorf("NegativeBranchStep2Op: unknown field %q", field)
	}
	val, ok := value.(*int)
	if !ok {
		return fmt.Errorf("NegativeBranchStep2Op: field in expected *int, got %T", value)
	}
	op.in = val
	return nil
}

func (op *NegativeBranchStep2Op) ResetFields() {
	op.in = nil
	op.out = 0
}

func init() {
	if err := operator.RegisterOp[NegativeBranchStep2Op](); err != nil {
		log.Fatalf("RegisterOp[NegativeStep2Op] error: %v", err)
	}
}
