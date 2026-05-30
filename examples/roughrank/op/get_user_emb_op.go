package op

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/operator"
)

const (
	embDim = 10
)

type GetUserEmbOp struct {
	userId  *int64    `dag:"input"`
	userEmb []float64 `dag:"output"`
}

func (op *GetUserEmbOp) Setup(params *config.Params) error {
	return nil
}

func (op *GetUserEmbOp) Run(ctx context.Context) error {
	if op.userId == nil {
		return fmt.Errorf("GetUserEmbOp: missing required input 'userId'")
	}
	// get user embed logic here
	for i := 0; i < embDim; i++ {
		op.userEmb = append(op.userEmb, rand.Float64())
	}
	return nil
}

func (op *GetUserEmbOp) Reset() error {
	return nil
}

func init() {
	if err := operator.RegisterOp[GetUserEmbOp](); err != nil {
		log.Fatalf("RegisterOp[GetUserEmbOp] error: %v", err)
	}
}
