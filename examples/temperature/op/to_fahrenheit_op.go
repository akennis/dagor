package op

import (
	"context"
	"fmt"
	"log"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/operator"
)

// ToFahrenheitOp converts a Celsius temperature (°C) to Fahrenheit (°F).
type ToFahrenheitOp struct {
	Celsius    *float64
	Fahrenheit float64
}

func (op *ToFahrenheitOp) Setup(_ *config.Params) error { return nil }

func (op *ToFahrenheitOp) Run(_ context.Context) error {
	if op.Celsius == nil {
		return fmt.Errorf("ToFahrenheitOp: Celsius input is nil")
	}
	op.Fahrenheit = *op.Celsius*9.0/5.0 + 32.0
	return nil
}

func (op *ToFahrenheitOp) Reset() error { return nil }

func (op *ToFahrenheitOp) InputFields() map[string]any {
	return map[string]any{"Celsius": &op.Celsius}
}

func (op *ToFahrenheitOp) OutputFields() map[string]any {
	return map[string]any{"Fahrenheit": &op.Fahrenheit}
}

func (op *ToFahrenheitOp) SetInputField(field string, value any) error {
	if field == "Celsius" {
		v, ok := value.(*float64)
		if !ok {
			return fmt.Errorf("ToFahrenheitOp: expected *float64 for field %q, got %T", field, value)
		}
		op.Celsius = v
	}
	return nil
}

func (op *ToFahrenheitOp) ResetFields() {
	op.Celsius = nil
	op.Fahrenheit = 0
}

func init() {
	if err := operator.RegisterOp[ToFahrenheitOp](); err != nil {
		log.Fatalf("RegisterOp[ToFahrenheitOp] error: %v", err)
	}
}
