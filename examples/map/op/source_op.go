package op

import (
	"context"
	"log"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/operator"
)

// SourceOp produces a fixed slice of integers configured via params.
// Params: {"values": [1, 2, 3, 4, 5]}
type SourceOp struct {
	Items []int `dag:"output"`
}

func (op *SourceOp) Setup(params *config.Params) error {
	raw := params.GetArrayInt64("values")
	// Convert int64 → int for convenience.
	op.Items = make([]int, len(raw))
	for i, v := range raw {
		op.Items[i] = int(v)
	}
	return nil
}

func (op *SourceOp) Run(_ context.Context) error {
	// Items already set in Setup; nothing more to do.
	return nil
}

func (op *SourceOp) Reset() error { return nil }

func (op *SourceOp) InputFields() map[string]any { return map[string]any{} }

func (op *SourceOp) OutputFields() map[string]any {
	return map[string]any{"Items": &op.Items}
}

func (op *SourceOp) SetInputField(_ string, _ any) error { return nil }

func (op *SourceOp) ResetFields() { op.Items = nil }

func init() {
	if err := operator.RegisterOp[SourceOp](); err != nil {
		log.Fatalf("RegisterOp[SourceOp] error: %v", err)
	}
}
