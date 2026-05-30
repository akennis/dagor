package op

import (
	"log"

	"github.com/akennis/dagor/operator"
	"github.com/akennis/dagor/operator/builtin"
)

type mathAKey struct{}
type mathBKey struct{}

// MathAKey and MathBKey are the context keys for the two integer operands.
// Inject them before calling eng.Run:
//
//	ctx = context.WithValue(ctx, op.MathAKey, 10)
//	ctx = context.WithValue(ctx, op.MathBKey, 20)
var (
	MathAKey = mathAKey{}
	MathBKey = mathBKey{}
)

func init() {
	entries := []struct {
		name    string
		factory func() operator.IOperator
	}{
		{"MathAOp", builtin.ContextValFactory[int](mathAKey{})},
		{"MathBOp", builtin.ContextValFactory[int](mathBKey{})},
	}
	for _, e := range entries {
		if err := operator.RegisterOpFactory(e.name, e.factory); err != nil {
			log.Fatalf("register %s: %v", e.name, err)
		}
	}
}
