package op

import (
	"context"
	"fmt"
	"log"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/operator"
)

// DoubleOp doubles a single integer element received via the "item" wire.
// It is used as a sub-graph operator inside a map node.
type DoubleOp struct {
	In  *int `dag:"input"`
	Out int  `dag:"output"`
}

func (op *DoubleOp) Setup(_ *config.Params) error { return nil }

func (op *DoubleOp) Run(_ context.Context) error {
	if op.In == nil {
		return fmt.Errorf("DoubleOp: input is nil")
	}
	op.Out = *op.In * 2
	return nil
}

func (op *DoubleOp) Reset() error { return nil }

func (op *DoubleOp) InputFields() map[string]any {
	return map[string]any{"In": &op.In}
}

func (op *DoubleOp) OutputFields() map[string]any {
	return map[string]any{"Out": &op.Out}
}

func (op *DoubleOp) SetInputField(field string, value any) error {
	if field == "In" {
		v, ok := value.(*int)
		if !ok {
			return fmt.Errorf("DoubleOp: expected *int for field %q, got %T", field, value)
		}
		op.In = v
	}
	return nil
}

func (op *DoubleOp) ResetFields() {
	op.In = nil
	op.Out = 0
}

func init() {
	if err := operator.RegisterOp[DoubleOp](); err != nil {
		log.Fatalf("RegisterOp[DoubleOp] error: %v", err)
	}
}
