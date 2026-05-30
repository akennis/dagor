package op

import (
	"context"
	"log"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/operator"
)

// SourceOp produces a slice of integers configured via params.
// Params: {"scores": [45, 72, 55, 88, ...]}
type SourceOp struct {
	Scores []int
}

func (op *SourceOp) Setup(params *config.Params) error {
	raw := params.GetArrayInt64("scores")
	op.Scores = make([]int, len(raw))
	for i, v := range raw {
		op.Scores[i] = int(v)
	}
	return nil
}

func (op *SourceOp) Run(_ context.Context) error { return nil }

func (op *SourceOp) Reset() error { return nil }

func (op *SourceOp) InputFields() map[string]any { return map[string]any{} }

func (op *SourceOp) OutputFields() map[string]any {
	return map[string]any{"Scores": &op.Scores}
}

func (op *SourceOp) SetInputField(_ string, _ any) error { return nil }

func (op *SourceOp) ResetFields() { op.Scores = nil }

func init() {
	if err := operator.RegisterOp[SourceOp](); err != nil {
		log.Fatalf("RegisterOp[SourceOp] error: %v", err)
	}
}
