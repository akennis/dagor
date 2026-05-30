package op

import (
	"context"
	"fmt"
	"log"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/examples/roughrank/model"
	"github.com/akennis/dagor/operator"
)

type RespOp struct {
	userId *int64          `dag:"input"`
	scores *[]*model.Score `dag:"input"`
	resp   model.Response  `dag:"output"`
}

func (op *RespOp) Setup(params *config.Params) error {
	return nil
}

func (op *RespOp) Run(ctx context.Context) error {
	if op.userId == nil || op.scores == nil {
		return fmt.Errorf("RespOp: missing required input 'userId' or 'scores'")
	}

	userId := *op.userId
	scores := *op.scores

	op.resp.UserId = userId
	for _, score := range scores {
		op.resp.Streams = append(op.resp.Streams, &model.Stream{
			StreamId: score.StreamId,
			Score:    score.Score,
		})
	}
	return nil
}

func (op *RespOp) Reset() error {
	return nil
}

func init() {
	if err := operator.RegisterOp[RespOp](); err != nil {
		log.Fatalf("RegisterOp[RespOp] error: %v", err)
	}
}
