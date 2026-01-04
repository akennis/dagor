package op

import (
	"context"
	"fmt"
	"log"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/operator"
)

type ConstOp struct {
	out int `dag:"output"`

	// params
	in int
}

func (op *ConstOp) Setup(params *config.Params) error {
	if params == nil {
		return fmt.Errorf("params is required")
	}
	op.in = params.GetInt("in", 0)
	return nil
}

func (op *ConstOp) Run(ctx context.Context) error {
	// pass the input to the output
	op.out = op.in
	return nil
}

func (op *ConstOp) Reset() error {
	op.in = 0 // reset in
	return nil
}

func init() {
	if err := operator.RegisterOp[ConstOp](); err != nil {
		log.Fatalf("RegisterOp[ConstOp] error: %v", err)
	}
}
