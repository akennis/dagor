package op

import (
	"context"
	"fmt"
	"log"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/operator"
)

type AddOp struct {
	a   *int `dag:"input"`
	b   *int `dag:"input"`
	sum int  `dag:"output"`
}

// Setup parses and validates params and setup internal fields.
// It will be called before Run.
// It should return error if the params are invalid.
func (op *AddOp) Setup(params *config.Params) error {
	return nil
}

// Run executes the operator.
// It should check input fields and do the business logic.
// It should return error if the execution fails.
func (op *AddOp) Run(ctx context.Context) error {
	if op.a == nil || op.b == nil {
		return fmt.Errorf("AddOp: missing required input 'a' or 'b'")
	}
	op.sum = *op.a + *op.b
	return nil
}

// Reset resets the operator state and clear internal fields in order to reuse next time.
// It will be called after Run.
func (op *AddOp) Reset() error {
	return nil
}

func init() {
	// register operator
	if err := operator.RegisterOp[AddOp](); err != nil {
		log.Fatalf("RegisterOp[AddOp] error: %v", err)
	}
}
