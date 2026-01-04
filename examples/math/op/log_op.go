package op

import (
	"context"
	"fmt"
	"log"
	"math"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/operator"
)

type LogOp struct {
	// input output fields
	x      *int    `dag:"input"`
	result float64 `dag:"output"`

	// params
	base int
}

func (op *LogOp) Setup(params *config.Params) error {
	if params == nil {
		return fmt.Errorf("params is required")
	}
	op.base = params.GetInt("base", 10)
	if op.base <= 0 || op.base == 1 {
		return fmt.Errorf("LogOp: base must be greater than 0 and not equal to 1")
	}
	return nil
}

func (op *LogOp) Run(ctx context.Context) error {
	if op.x == nil {
		return fmt.Errorf("LogOp: missing required input 'x'")
	}
	if *op.x <= 0 {
		return fmt.Errorf("LogOp: x must be greater than 0")
	}
	op.result = math.Log(float64(*op.x)) / math.Log(float64(op.base))
	return nil
}

func (op *LogOp) Reset() error {
	op.base = 0 // reset in
	return nil
}

func init() {
	if err := operator.RegisterOp[LogOp](); err != nil {
		log.Fatalf("RegisterOp[LogOp] error: %v", err)
	}
}
