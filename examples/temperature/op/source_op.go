package op

import (
	"context"
	"fmt"
	"log"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/operator"
)

// TempSourceOp emits a fixed slice of Celsius temperatures configured via params.
// Params: {"temps": [-10.0, 0.0, 20.0, 37.0, 100.0]}
type TempSourceOp struct {
	Temps []float64
}

func (op *TempSourceOp) Setup(params *config.Params) error {
	op.Temps = params.GetArrayFloat64("temps")
	if len(op.Temps) == 0 {
		return fmt.Errorf("TempSourceOp: params.temps is required")
	}
	return nil
}

func (op *TempSourceOp) Run(_ context.Context) error { return nil }

func (op *TempSourceOp) Reset() error { return nil }

func (op *TempSourceOp) InputFields() map[string]any { return map[string]any{} }

func (op *TempSourceOp) OutputFields() map[string]any {
	return map[string]any{"Temps": &op.Temps}
}

func (op *TempSourceOp) SetInputField(_ string, _ any) error { return nil }

func (op *TempSourceOp) ResetFields() { op.Temps = nil }

func init() {
	if err := operator.RegisterOp[TempSourceOp](); err != nil {
		log.Fatalf("RegisterOp[TempSourceOp] error: %v", err)
	}
}
