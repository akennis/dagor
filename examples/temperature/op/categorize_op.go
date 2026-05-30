package op

import (
	"context"
	"fmt"
	"log"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/operator"
)

// CategorizeOp classifies a Fahrenheit temperature into a human-readable label.
// <32°F → "freezing", [32,60) → "cold", [60,86) → "warm", ≥86°F → "hot"
type CategorizeOp struct {
	Temp     *float64
	Category string
}

func (op *CategorizeOp) Setup(_ *config.Params) error { return nil }

func (op *CategorizeOp) Run(_ context.Context) error {
	if op.Temp == nil {
		return fmt.Errorf("CategorizeOp: Temp input is nil")
	}
	switch {
	case *op.Temp >= 86:
		op.Category = "hot"
	case *op.Temp >= 60:
		op.Category = "warm"
	case *op.Temp >= 32:
		op.Category = "cold"
	default:
		op.Category = "freezing"
	}
	return nil
}

func (op *CategorizeOp) Reset() error { return nil }

func (op *CategorizeOp) InputFields() map[string]any {
	return map[string]any{"Temp": &op.Temp}
}

func (op *CategorizeOp) OutputFields() map[string]any {
	return map[string]any{"Category": &op.Category}
}

func (op *CategorizeOp) SetInputField(field string, value any) error {
	if field == "Temp" {
		v, ok := value.(*float64)
		if !ok {
			return fmt.Errorf("CategorizeOp: expected *float64 for field %q, got %T", field, value)
		}
		op.Temp = v
	}
	return nil
}

func (op *CategorizeOp) ResetFields() {
	op.Temp = nil
	op.Category = ""
}

func init() {
	if err := operator.RegisterOp[CategorizeOp](); err != nil {
		log.Fatalf("RegisterOp[CategorizeOp] error: %v", err)
	}
}
