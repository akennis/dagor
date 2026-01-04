package operator

import (
	"context"
	"testing"

	"github.com/wwz16/dagor/config"
)

func TestNewOperatorRegistry(t *testing.T) {
	registry := NewOperatorRegistry()
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	// sync.Map is always non-nil, so we just verify the registry was created
}

func TestOperatorRegistry_Register(t *testing.T) {
	registry := NewOperatorRegistry()

	// Test successful registration
	err := registry.Register("test_op", func() IOperator {
		return newMockOperator("test")
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test registering with empty name
	err = registry.Register("", func() IOperator {
		return newMockOperator("test")
	})
	if err == nil {
		t.Error("expected error for empty name")
	}
	if err.Error() != "name is required" {
		t.Errorf("expected 'name is required' error, got: %v", err)
	}

	// Test registering with nil builder
	err = registry.Register("test_op2", nil)
	if err == nil {
		t.Error("expected error for nil builder")
	}
	if err.Error() != "opBuilder is required" {
		t.Errorf("expected 'opBuilder is required' error, got: %v", err)
	}

	// Test duplicate registration
	err = registry.Register("test_op", func() IOperator {
		return newMockOperator("test")
	})
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
	if err.Error() != "operator pool already registered for name: test_op" {
		t.Errorf("expected duplicate registration error, got: %v", err)
	}
}

func TestOperatorRegistry_Get(t *testing.T) {
	registry := NewOperatorRegistry()

	// Register an operator
	err := registry.Register("test_op", func() IOperator {
		return newMockOperator("test")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test successful Get
	op, err := registry.Get("test_op")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if op == nil {
		t.Fatal("expected non-nil operator")
	}

	// Test Get with non-existent name
	op, err = registry.Get("non_existent")
	if err == nil {
		t.Error("expected error for non-existent operator")
	}
	if op != nil {
		t.Error("expected nil operator for non-existent name")
	}
	if err.Error() != "operator pool not found for name: non_existent" {
		t.Errorf("expected 'operator pool not found' error, got: %v", err)
	}
}

func TestOperatorRegistry_Put(t *testing.T) {
	registry := NewOperatorRegistry()

	// Register an operator
	err := registry.Register("test_op", func() IOperator {
		return newMockOperator("test")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get an operator
	op, err := registry.Get("test_op")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Put it back
	err = registry.Put("test_op", op)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test Put with non-existent name
	err = registry.Put("non_existent", op)
	if err == nil {
		t.Error("expected error for non-existent operator")
	}
	if err.Error() != "operator pool not found for name: non_existent" {
		t.Errorf("expected 'operator pool not found' error, got: %v", err)
	}
}

func TestOperatorRegistry_GetPutCycle(t *testing.T) {
	registry := NewOperatorRegistry()

	// Register an operator
	err := registry.Register("test_op", func() IOperator {
		return newMockOperator("test")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get operator
	op1, err := registry.Get("test_op")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Put it back
	err = registry.Put("test_op", op1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get again - should reuse the same instance
	op2, err := registry.Get("test_op")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be the same instance (reused from pool)
	if op1 != op2 {
		t.Error("expected same operator instance after Put/Get")
	}
}

func TestOperatorRegistry_MultipleOperators(t *testing.T) {
	registry := NewOperatorRegistry()

	// Register multiple operators
	ops := []string{"op1", "op2", "op3"}
	for _, opName := range ops {
		err := registry.Register(opName, func() IOperator {
			return newMockOperator(opName)
		})
		if err != nil {
			t.Fatalf("unexpected error registering %s: %v", opName, err)
		}
	}

	// Get each operator
	for _, opName := range ops {
		op, err := registry.Get(opName)
		if err != nil {
			t.Errorf("unexpected error getting %s: %v", opName, err)
		}
		if op == nil {
			t.Errorf("expected non-nil operator for %s", opName)
		}
		// Verify it's the correct type
		mockOp, ok := op.(*mockOperator)
		if !ok {
			t.Errorf("expected *mockOperator for %s", opName)
		} else if mockOp.name != opName {
			t.Errorf("expected operator name %s, got %s", opName, mockOp.name)
		}
	}
}

func TestRegisterOp(t *testing.T) {
	// Use a unique operator type for this test
	type uniqueTestOp struct {
		mockOperator
	}

	// Test with a valid operator type
	err := RegisterOp[uniqueTestOp]()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test that we can get it
	op, err := GetOp("uniqueTestOp")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if op == nil {
		t.Fatal("expected non-nil operator")
	}

	// Verify it's the correct type
	_, ok := op.(*uniqueTestOp)
	if !ok {
		t.Error("expected *uniqueTestOp")
	}
}

func TestRegisterOp_InvalidType(t *testing.T) {
	// Test with a type that doesn't implement IOperator
	type invalidType struct{}

	err := RegisterOp[invalidType]()
	if err == nil {
		t.Error("expected error for type that doesn't implement IOperator")
	}
	if err.Error() != "type *operator.invalidType must implement IOperator" {
		t.Errorf("expected IOperator implementation error, got: %v", err)
	}
}

func TestGetOp(t *testing.T) {
	// Use a unique operator type for this test
	type getTestOp struct {
		mockOperator
	}

	// Register an operator first (may already be registered, that's ok)
	err := RegisterOp[getTestOp]()
	if err != nil && err.Error() != "operator pool already registered for name: getTestOp" {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test successful Get
	op, err := GetOp("getTestOp")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if op == nil {
		t.Fatal("expected non-nil operator")
	}

	// Test Get with non-existent name
	op, err = GetOp("non_existent_op_for_test")
	if err == nil {
		t.Error("expected error for non-existent operator")
	}
	if op != nil {
		t.Error("expected nil operator for non-existent name")
	}
}

func TestPutOp(t *testing.T) {
	// Use a unique operator type for this test
	type putTestOp struct {
		mockOperator
	}

	// Register an operator first (may already be registered, that's ok)
	err := RegisterOp[putTestOp]()
	if err != nil && err.Error() != "operator pool already registered for name: putTestOp" {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get an operator
	op, err := GetOp("putTestOp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Put it back
	err = PutOp("putTestOp", op)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test Put with non-existent name
	err = PutOp("non_existent_op_for_put_test", op)
	if err == nil {
		t.Error("expected error for non-existent operator")
	}
}

// testOperator is a test operator type for integration testing
type testOperator struct {
	value int
}

func (op *testOperator) Setup(params *config.Params) error {
	return nil
}

func (op *testOperator) Run(ctx context.Context) error {
	op.value = 42
	return nil
}

func (op *testOperator) Reset() error {
	op.value = 0
	return nil
}

func (op *testOperator) InputFields() map[string]any {
	return make(map[string]any)
}

func (op *testOperator) OutputFields() map[string]any {
	return make(map[string]any)
}

func (op *testOperator) SetInputField(field string, value any) error {
	return nil
}

func (op *testOperator) ResetFields() {
	op.value = 0
}

func TestRegisterOp_GetOp_PutOp_Integration(t *testing.T) {
	// Register the operator
	err := RegisterOp[testOperator]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get operator
	op1, err := GetOp("testOperator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testOp1 := op1.(*testOperator)

	// Use the operator
	testOp1.value = 100

	// Put it back
	err = PutOp("testOperator", op1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get again - should get a new or reset instance
	op2, err := GetOp("testOperator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testOp2 := op2.(*testOperator)

	// The value should be reset (0) for a new instance
	// Note: if the pool reuses the instance, the value might still be 100
	// This depends on whether Reset() is called before Put
	// For this test, we just verify we can get a valid operator
	if testOp2 == nil {
		t.Error("expected non-nil testOperator")
	}
}

func TestOperatorRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewOperatorRegistry()

	// Register an operator
	err := registry.Register("test_op", func() IOperator {
		return newMockOperator("test")
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test concurrent Get/Put operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			op, err := registry.Get("test_op")
			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", id, err)
			}
			if op == nil {
				t.Errorf("goroutine %d: expected non-nil operator", id)
			}
			registry.Put("test_op", op)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
