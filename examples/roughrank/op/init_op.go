package op

import (
	"context"
	"fmt"
	"log"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/examples/roughrank/model"
	"github.com/akennis/dagor/operator"
)

type InitOp struct {
	userId    int64   `dag:"output"`
	streamIds []int64 `dag:"output"`

	req *model.Request
}

func (op *InitOp) Setup(params *config.Params) error {
	op.req = &model.Request{
		UserId:    params.GetInt64("user_id", 0),
		StreamIds: params.GetArrayInt64("stream_ids"),
	}
	return nil
}

func (op *InitOp) Run(ctx context.Context) error {
	if op.req == nil {
		return fmt.Errorf("InitOp: missing required input 'req'")
	}
	op.userId = op.req.UserId
	op.streamIds = op.req.StreamIds
	return nil
}

func (op *InitOp) Reset() error {
	op.req = nil
	return nil
}

func init() {
	if err := operator.RegisterOp[InitOp](); err != nil {
		log.Fatalf("RegisterOp[InitOp] error: %v", err)
	}
}
