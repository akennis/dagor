package op

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/examples/roughrank/model"
	"github.com/wwz16/dagor/operator"
)

type GetStreamEmbsOp struct {
	streamIds  *[]int64           `dag:"input"`
	streamEmbs []*model.StreamEmb `dag:"output"`
}

func (op *GetStreamEmbsOp) Setup(params *config.Params) error {
	return nil
}

func (op *GetStreamEmbsOp) Run(ctx context.Context) error {
	if op.streamIds == nil {
		return fmt.Errorf("GetStreamEmbsOp: missing required input 'streamIds'")
	}
	streamIds := *op.streamIds
	for _, id := range streamIds {
		emb := op.getStreamEmb(id)
		op.streamEmbs = append(op.streamEmbs, &model.StreamEmb{Id: id, Emb: emb})
	}
	return nil
}

func (op *GetStreamEmbsOp) getStreamEmb(id int64) []float64 {
	emb := make([]float64, embDim)
	for i := 0; i < embDim; i++ {
		emb[i] = rand.Float64()
	}
	return emb
}

func (op *GetStreamEmbsOp) Reset() error {
	return nil
}

func init() {
	if err := operator.RegisterOp[GetStreamEmbsOp](); err != nil {
		log.Fatalf("RegisterOp[GetStreamEmbsOp] error: %v", err)
	}
}
