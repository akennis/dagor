package operator

import (
	"context"

	"github.com/wwz16/dagor/config"
)

// IOperator is the interface for all operators.
// It should be implemented by all operator classes.
type IOperator interface {
	// Setup parses and validates params and setup internal fields.
	// It will be called before Run.
	// It should return error if the params are invalid.
	Setup(params *config.Params) error

	// Run executes the operator.
	// It should check input fields and do the business logic.
	// It should return error if the execution fails.
	Run(ctx context.Context) error

	// Reset resets the operator state and clear internal fields in order to reuse next time.
	// It will be called after Run.
	Reset() error

	// InputFields returns the input fields of the operator.
	// The key is the input field name, the value is the runtime field value.
	InputFields() map[string]any

	// OutputFields returns the output fields of the operator.
	// The key is the output field name, the value is the runtime field value.
	OutputFields() map[string]any

	// SetInputField sets the input field of the operator.
	SetInputField(field string, value any) error

	// ResetFields resets the fields of the operator.
	ResetFields()
}
