package op

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/examples/roughrank/model"
	"github.com/akennis/dagor/operator"
)

type CalcSimOp struct {
	userEmb    *[]float64          `dag:"input"`
	streamEmbs *[]*model.StreamEmb `dag:"input"`
	scores     []*model.Score      `dag:"output"`
}

func (op *CalcSimOp) Setup(params *config.Params) error {
	return nil
}

func (op *CalcSimOp) Run(ctx context.Context) error {
	if op.userEmb == nil || op.streamEmbs == nil {
		return fmt.Errorf("CalcSimOp: missing required input 'userEmb' or 'streamEmbs'")
	}
	userEmb := *op.userEmb
	streamEmbs := *op.streamEmbs

	// Calculate the similarity between userEmb and each streamEmb
	for _, streamEmb := range streamEmbs {
		score := op.calcSim(userEmb, streamEmb.Emb)
		op.scores = append(op.scores, &model.Score{StreamId: streamEmb.Id, Score: score})
	}

	// Sort the scores
	op.sortScores()

	return nil
}

// calcSim calculates the similarity between userEmb and streamEmb.
// using dot method to calculate the similarity of two vectors.
// dot = sum(userEmb[i] * streamEmb[i])
// sim = dot / (||userEmb|| * ||streamEmb||)
func (op *CalcSimOp) calcSim(userEmb, streamEmb []float64) float64 {
	if len(userEmb) == 0 || len(userEmb) != len(streamEmb) {
		return 0
	}

	// Calculate the dot product
	dot := 0.0
	for i := 0; i < len(userEmb); i++ {
		dot += userEmb[i] * streamEmb[i]
	}

	// Calculate the norm of the vectors
	userEmbNorm := 0.0
	streamEmbNorm := 0.0
	for i := 0; i < len(userEmb); i++ {
		userEmbNorm += userEmb[i] * userEmb[i]
		streamEmbNorm += streamEmb[i] * streamEmb[i]
	}
	userEmbNorm = math.Sqrt(userEmbNorm)
	streamEmbNorm = math.Sqrt(streamEmbNorm)
	if userEmbNorm == 0 || streamEmbNorm == 0 {
		return 0
	}

	// Calculate the cosine similarity
	cosSim := dot / (userEmbNorm * streamEmbNorm)
	return cosSim
}

func (op *CalcSimOp) sortScores() {
	sort.Slice(op.scores, func(i, j int) bool {
		return op.scores[i].Score > op.scores[j].Score
	})
}

func (op *CalcSimOp) Reset() error {
	return nil
}

func init() {
	if err := operator.RegisterOp[CalcSimOp](); err != nil {
		log.Fatalf("RegisterOp[CalcSimOp] error: %v", err)
	}
}
