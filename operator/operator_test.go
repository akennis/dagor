package operator

import (
	"context"
	"errors"
	"testing"

	"github.com/akennis/dagor/config"
)

// mockOperator is a mock implementation of IOperator for testing
type mockOperator struct {
	name         string
	setupCalled  bool
	runCalled    bool
	resetCalled  bool
	inputFields  map[string]any
	outputFields map[string]any
	setupErr     error
	runErr       error
	resetErr     error
}

func newMockOperator(name string) *mockOperator {
	return &mockOperator{
		name:         name,
		inputFields:  make(map[string]any),
		outputFields: make(map[string]any),
	}
}

func (m *mockOperator) Setup(params *config.Params) error {
	m.setupCalled = true
	return m.setupErr
}

func (m *mockOperator) Run(ctx context.Context) error {
	m.runCalled = true
	return m.runErr
}

func (m *mockOperator) Reset() error {
	m.resetCalled = true

	m.setupCalled = false
	m.runCalled = false
	m.setupErr = nil
	m.runErr = nil
	return m.resetErr
}

func (m *mockOperator) InputFields() map[string]any {
	return m.inputFields
}

func (m *mockOperator) OutputFields() map[string]any {
	return m.outputFields
}

func (m *mockOperator) SetInputField(field string, value any) error {
	m.inputFields[field] = value
	return nil
}

func (m *mockOperator) ResetFields() {
	m.inputFields = make(map[string]any)
	m.outputFields = make(map[string]any)
}

var errTestError = errors.New("test error")

// Test that mockOperator implements IOperator interface
func TestMockOperator_ImplementsIOperator(t *testing.T) {
	var _ IOperator = (*mockOperator)(nil)
}

func TestMockOperator_Setup(t *testing.T) {
	op := newMockOperator("test")
	params, _ := config.NewFromRaw([]byte(`{"key": "value"}`))

	err := op.Setup(params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !op.setupCalled {
		t.Error("expected Setup to be called")
	}
}

func TestMockOperator_Setup_WithError(t *testing.T) {
	op := newMockOperator("test")
	op.setupErr = errTestError
	params, _ := config.NewFromRaw([]byte(`{}`))

	err := op.Setup(params)
	if err != op.setupErr {
		t.Errorf("expected error %v, got %v", op.setupErr, err)
	}
	if !op.setupCalled {
		t.Error("expected Setup to be called")
	}
}

func TestMockOperator_Run(t *testing.T) {
	op := newMockOperator("test")
	ctx := context.Background()

	err := op.Run(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !op.runCalled {
		t.Error("expected Run to be called")
	}
}

func TestMockOperator_Run_WithError(t *testing.T) {
	op := newMockOperator("test")
	op.runErr = errTestError
	ctx := context.Background()

	err := op.Run(ctx)
	if err != op.runErr {
		t.Errorf("expected error %v, got %v", op.runErr, err)
	}
	if !op.runCalled {
		t.Error("expected Run to be called")
	}
}

func TestMockOperator_Reset(t *testing.T) {
	op := newMockOperator("test")

	err := op.Reset()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !op.resetCalled {
		t.Error("expected Reset to be called")
	}
}

func TestMockOperator_Reset_WithError(t *testing.T) {
	op := newMockOperator("test")
	op.resetErr = errTestError

	err := op.Reset()
	if err != op.resetErr {
		t.Errorf("expected error %v, got %v", op.resetErr, err)
	}
	if !op.resetCalled {
		t.Error("expected Reset to be called")
	}
}

func TestMockOperator_InputFields(t *testing.T) {
	op := newMockOperator("test")

	fields := op.InputFields()
	if fields == nil {
		t.Fatal("expected non-nil input fields map")
	}
	if len(fields) != 0 {
		t.Errorf("expected empty input fields, got %d", len(fields))
	}
}

func TestMockOperator_OutputFields(t *testing.T) {
	op := newMockOperator("test")

	fields := op.OutputFields()
	if fields == nil {
		t.Fatal("expected non-nil output fields map")
	}
	if len(fields) != 0 {
		t.Errorf("expected empty output fields, got %d", len(fields))
	}
}

func TestMockOperator_SetInputField(t *testing.T) {
	op := newMockOperator("test")

	err := op.SetInputField("field1", "value1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	fields := op.InputFields()
	if fields["field1"] != "value1" {
		t.Errorf("expected field1='value1', got %v", fields["field1"])
	}

	// Set another field
	err = op.SetInputField("field2", 42)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if fields["field2"] != 42 {
		t.Errorf("expected field2=42, got %v", fields["field2"])
	}
}

func TestMockOperator_SetInputField_Multiple(t *testing.T) {
	op := newMockOperator("test")

	// Set multiple fields
	fields := []struct {
		name  string
		value any
	}{
		{"a", 1},
		{"b", "test"},
		{"c", true},
		{"d", 3.14},
	}

	for _, f := range fields {
		err := op.SetInputField(f.name, f.value)
		if err != nil {
			t.Errorf("unexpected error setting %s: %v", f.name, err)
		}
	}

	// Verify all fields are set
	inputFields := op.InputFields()
	if len(inputFields) != len(fields) {
		t.Errorf("expected %d input fields, got %d", len(fields), len(inputFields))
	}

	for _, f := range fields {
		if inputFields[f.name] != f.value {
			t.Errorf("expected %s=%v, got %v", f.name, f.value, inputFields[f.name])
		}
	}
}

func TestMockOperator_ResetFields(t *testing.T) {
	op := newMockOperator("test")

	// Set some fields
	op.SetInputField("field1", "value1")
	op.outputFields["output1"] = "result1"

	// Verify fields are set
	if len(op.InputFields()) == 0 {
		t.Error("expected input fields to be set")
	}
	if len(op.OutputFields()) == 0 {
		t.Error("expected output fields to be set")
	}

	// Reset fields
	op.ResetFields()

	// Verify fields are cleared
	if len(op.InputFields()) != 0 {
		t.Error("expected input fields to be cleared")
	}
	if len(op.OutputFields()) != 0 {
		t.Error("expected output fields to be cleared")
	}
}

func TestMockOperator_FullLifecycle(t *testing.T) {
	op := newMockOperator("test")
	ctx := context.Background()
	params, _ := config.NewFromRaw([]byte(`{"key": "value"}`))

	// Setup
	err := op.Setup(params)
	if err != nil {
		t.Errorf("unexpected error in Setup: %v", err)
	}
	if !op.setupCalled {
		t.Error("expected Setup to be called")
	}

	// Set input fields
	err = op.SetInputField("input1", "value1")
	if err != nil {
		t.Errorf("unexpected error setting input field: %v", err)
	}

	// Run
	err = op.Run(ctx)
	if err != nil {
		t.Errorf("unexpected error in Run: %v", err)
	}
	if !op.runCalled {
		t.Error("expected Run to be called")
	}

	// Reset
	err = op.Reset()
	if err != nil {
		t.Errorf("unexpected error in Reset: %v", err)
	}
	if !op.resetCalled {
		t.Error("expected Reset to be called")
	}

	// ResetFields
	op.ResetFields()
	if len(op.InputFields()) != 0 {
		t.Error("expected input fields to be cleared after ResetFields")
	}
}
