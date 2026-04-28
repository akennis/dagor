package op

import (
	"context"
	"fmt"
	"log"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/operator"
)

// NegativeBranchOp is the negative value operator used in the coalesce demo.
// It negates its integer input, converting a negative value to a positive one
type NegativeBranchOp struct {
	in  *int `dag:"input"`
	out int  `dag:"output"`
}

func (op *NegativeBranchOp) Setup(_ *config.Params) error { return nil }
func (op *NegativeBranchOp) Reset() error                 { return nil }

func (op *NegativeBranchOp) Run(_ context.Context) error {
	if op.in == nil {
		return fmt.Errorf("NegativeBranchOp: missing required input")
	}
	op.out = -(*op.in)
	return nil
}

func (op *NegativeBranchOp) InputFields() map[string]any  { return map[string]any{"in": &op.in} }
func (op *NegativeBranchOp) OutputFields() map[string]any { return map[string]any{"out": &op.out} }

func (op *NegativeBranchOp) SetInputField(field string, value any) error {
	if value == nil {
		return nil
	}
	if field != "in" {
		return fmt.Errorf("NegativeBranchOp: unknown field %q", field)
	}
	val, ok := value.(*int)
	if !ok {
		return fmt.Errorf("NegativeBranchOp: field in expected *int, got %T", value)
	}
	op.in = val
	return nil
}

func (op *NegativeBranchOp) ResetFields() {
	op.in = nil
	op.out = 0
}

func init() {
	if err := operator.RegisterOp[NegativeBranchOp](); err != nil {
		log.Fatalf("RegisterOp[NegativeBranchOp] error: %v", err)
	}
}
