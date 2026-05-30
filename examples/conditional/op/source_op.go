package op

import (
	"context"
	"log"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/operator"
)

// SourceOp outputs a configurable integer value.
type SourceOp struct {
	out int `dag:"output"`

	// params
	val int
}

func (op *SourceOp) Setup(params *config.Params) error {
	op.val = params.GetInt("value", 0)
	return nil
}

func (op *SourceOp) Run(ctx context.Context) error {
	op.out = op.val
	return nil
}

func (op *SourceOp) Reset() error {
	return nil
}

func (op *SourceOp) InputFields() map[string]any {
	return map[string]any{}
}

func (op *SourceOp) OutputFields() map[string]any {
	return map[string]any{
		"out": &op.out,
	}
}

func (op *SourceOp) SetInputField(field string, value any) error {
	return nil
}

func (op *SourceOp) ResetFields() {
	var zeroout int
	op.out = zeroout
}

func init() {
	if err := operator.RegisterOp[SourceOp](); err != nil {
		log.Fatalf("RegisterOp[SourceOp] error: %v", err)
	}
}
