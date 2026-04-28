package operator

import (
	"testing"
)

func TestNewOperatorPool(t *testing.T) {
	poolName := "test_pool"
	pool := NewOperatorPool(poolName, func() IOperator {
		return newMockOperator("test")
	})

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
	if pool.name != poolName {
		t.Errorf("expected pool name '%s', got '%s'", poolName, pool.name)
	}
	if pool.pool == nil {
		t.Error("expected non-nil sync.Pool")
	}
}

func TestOperatorPool_Get(t *testing.T) {
	pool := NewOperatorPool("test", func() IOperator {
		return newMockOperator("test")
	})

	// Get should return a new operator when pool is empty
	op1 := pool.Get()
	if op1 == nil {
		t.Fatal("expected non-nil operator")
	}

	// Get should return a new operator each time when pool is empty
	op2 := pool.Get()
	if op2 == nil {
		t.Fatal("expected non-nil operator")
	}

	// Operators should be different instances
	if op1 == op2 {
		t.Error("expected different operator instances")
	}
}

func TestOperatorPool_Put(t *testing.T) {
	pool := NewOperatorPool("test", func() IOperator {
		return newMockOperator("test")
	})

	op1 := pool.Get()
	if op1 == nil {
		t.Fatal("expected non-nil operator")
	}

	pool.Put(op1)

	// Get after Put must return a valid operator (sync.Pool may or may not reuse the same instance).
	op2 := pool.Get()
	if op2 == nil {
		t.Fatal("expected non-nil operator after Put/Get")
	}
}

func TestOperatorPool_GetPutCycle(t *testing.T) {
	pool := NewOperatorPool("test", func() IOperator {
		return newMockOperator("test")
	})

	ops := make([]IOperator, 5)
	for i := 0; i < 5; i++ {
		ops[i] = pool.Get()
		if ops[i] == nil {
			t.Fatalf("expected non-nil operator at index %d", i)
		}
	}

	for i := 0; i < 5; i++ {
		pool.Put(ops[i])
	}

	// Each Get after Put must return a valid operator.
	// sync.Pool does not guarantee pointer identity, so we only check non-nil.
	for i := 0; i < 5; i++ {
		op := pool.Get()
		if op == nil {
			t.Fatalf("expected non-nil operator at index %d after Put/Get cycle", i)
		}
	}
}

func TestOperatorPool_ConcurrentAccess(t *testing.T) {
	pool := NewOperatorPool("test", func() IOperator {
		return newMockOperator("test")
	})

	// Test concurrent Get/Put operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			op := pool.Get()
			if op == nil {
				t.Errorf("goroutine %d: expected non-nil operator", id)
			}
			pool.Put(op)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestOperatorPool_ResetBeforePut(t *testing.T) {
	pool := NewOperatorPool("test", func() IOperator {
		return newMockOperator("test")
	})

	op := pool.Get().(*mockOperator)

	// Set some state
	op.inputFields["test"] = "value"
	op.outputFields["result"] = 42

	// Reset fields
	op.ResetFields()

	// Verify fields are reset
	if len(op.inputFields) != 0 {
		t.Error("expected input fields to be reset")
	}
	if len(op.outputFields) != 0 {
		t.Error("expected output fields to be reset")
	}

	// Put back to pool
	pool.Put(op)

	// Get again and verify it's clean
	op2 := pool.Get().(*mockOperator)
	if len(op2.inputFields) != 0 {
		t.Error("expected input fields to be empty after Get")
	}
	if len(op2.outputFields) != 0 {
		t.Error("expected output fields to be empty after Get")
	}
}
