package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/operator"
)

// mockOperator is a mock implementation of IOperator for testing
type mockOperator struct {
	name string
}

func (m *mockOperator) Setup(params *config.Params) error { return nil }
func (m *mockOperator) Run(ctx context.Context) error     { return nil }
func (m *mockOperator) Reset() error                      { return nil }
func (m *mockOperator) InputFields() map[string]any       { return nil }
func (m *mockOperator) OutputFields() map[string]any      { return nil }
func (m *mockOperator) SetInputField(field string, value any) error {
	return nil
}
func (m *mockOperator) ResetFields() {}

// Test that mockOperator implements IOperator interface
func TestMockOperator_ImplementsIOperator(t *testing.T) {
	var _ operator.IOperator = (*mockOperator)(nil)
}

func TestNewGraphStatus(t *testing.T) {
	status := NewGraphStatus()
	if status == nil {
		t.Fatal("expected non-nil GraphStatus")
	}

	if status.State() != GraphStateInit {
		t.Errorf("expected initial state to be GraphStateInit, got %v", status.State())
	}

	if status.ops == nil {
		t.Error("expected ops map to be initialized")
	}
	if status.fieldValues == nil {
		t.Error("expected fieldValues map to be initialized")
	}
	if status.inDegrees == nil {
		t.Error("expected inDegrees map to be initialized")
	}
	if status.done == nil {
		t.Error("expected done channel to be initialized")
	}
}

func TestState(t *testing.T) {
	status := NewGraphStatus()

	if status.State() != GraphStateInit {
		t.Errorf("expected initial state to be GraphStateInit, got %v", status.State())
	}

	status.SetState(GraphStateRunning)
	if status.State() != GraphStateRunning {
		t.Errorf("expected state to be GraphStateRunning, got %v", status.State())
	}

	status.SetState(GraphStateFinished)
	if status.State() != GraphStateFinished {
		t.Errorf("expected state to be GraphStateFinished, got %v", status.State())
	}
}

func TestState_Concurrent(t *testing.T) {
	status := NewGraphStatus()
	var wg sync.WaitGroup
	iterations := 100

	// Test concurrent SetState calls
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			status.SetState(GraphStateRunning)
			status.SetState(GraphStateFinished)
			status.SetState(GraphStateInit)
		}()
	}
	wg.Wait()

	// State should be one of the valid states
	state := status.State()
	if state != GraphStateInit && state != GraphStateRunning && state != GraphStateFinished {
		t.Errorf("expected state to be one of the valid states, got %v", state)
	}
}

func TestSetError(t *testing.T) {
	status := NewGraphStatus()

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	status.SetError(err1)
	if status.Error() != err1 {
		t.Errorf("expected error to be err1, got %v", status.Error())
	}

	// Setting a second error should not overwrite the first
	status.SetError(err2)
	if status.Error() != err1 {
		t.Errorf("expected error to still be err1, got %v", status.Error())
	}

	// Setting nil error should not change anything
	status.SetError(nil)
	if status.Error() != err1 {
		t.Errorf("expected error to still be err1 after setting nil, got %v", status.Error())
	}
}

func TestSetError_NotifiesDone(t *testing.T) {
	status := NewGraphStatus()

	err := errors.New("test error")
	status.SetError(err)

	// Done channel should be closed
	select {
	case <-status.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("expected done channel to be closed after SetError")
	}
}

func TestSetError_MultipleCalls(t *testing.T) {
	status := NewGraphStatus()

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	// First error should be stored
	status.SetError(err1)
	if status.Error() != err1 {
		t.Errorf("expected error to be err1, got %v", status.Error())
	}

	// Second error should not overwrite
	status.SetError(err2)
	if status.Error() != err1 {
		t.Errorf("expected error to still be err1, got %v", status.Error())
	}

	// Done channel should only be closed once
	select {
	case <-status.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("expected done channel to be closed")
	}
}

func TestSetError_Concurrent(t *testing.T) {
	const goroutines = 200
	for range 10 { // repeat to increase race window coverage
		status := NewGraphStatus()

		errs := make([]error, goroutines)
		for i := range errs {
			errs[i] = errors.New("concurrent error")
		}

		var wg sync.WaitGroup
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			i := i
			go func() {
				defer wg.Done()
				status.SetError(errs[i])
			}()
		}
		wg.Wait()

		got := status.Error()
		if got == nil {
			t.Fatal("expected a non-nil error after concurrent SetError calls")
		}
		found := false
		for _, e := range errs {
			if got == e {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("stored error %v is not one of the submitted errors", got)
		}

		select {
		case <-status.Done():
		default:
			t.Error("done channel should be closed after SetError")
		}
	}
}

func TestError_NoError(t *testing.T) {
	status := NewGraphStatus()

	if status.Error() != nil {
		t.Errorf("expected no error initially, got %v", status.Error())
	}
}

func TestSetVertexError(t *testing.T) {
	status := NewGraphStatus()

	v1, _ := graph.NewVertex("v1", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})
	v2, _ := graph.NewVertex("v2", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})

	err1 := errors.New("vertex error 1")
	err2 := errors.New("vertex error 2")

	status.SetVertexError(v1, err1)
	if status.VertexError(v1) != err1 {
		t.Errorf("expected vertex error to be err1, got %v", status.VertexError(v1))
	}

	status.SetVertexError(v2, err2)
	if status.VertexError(v2) != err2 {
		t.Errorf("expected vertex error to be err2, got %v", status.VertexError(v2))
	}

	// Setting nil error should not change anything
	status.SetVertexError(v1, nil)
	if status.VertexError(v1) != err1 {
		t.Errorf("expected vertex error to still be err1 after setting nil, got %v", status.VertexError(v1))
	}
}

func TestVertexError_NoError(t *testing.T) {
	status := NewGraphStatus()

	v, _ := graph.NewVertex("v1", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})

	if status.VertexError(v) != nil {
		t.Errorf("expected no vertex error initially, got %v", status.VertexError(v))
	}
}

func TestSetOp(t *testing.T) {
	status := NewGraphStatus()

	op1 := &mockOperator{name: "op1"}
	op2 := &mockOperator{name: "op2"}

	status.SetOp("v1", op1)
	status.SetOp("v2", op2)

	retrievedOp1, ok := status.Op("v1")
	if !ok {
		t.Error("expected op1 to be found")
	}
	if retrievedOp1 == nil {
		t.Error("expected op1 to be non-nil")
	}

	retrievedOp2, ok := status.Op("v2")
	if !ok {
		t.Error("expected op2 to be found")
	}
	if retrievedOp2 == nil {
		t.Error("expected op2 to be non-nil")
	}

	// Setting nil operator should not change anything
	status.SetOp("v1", nil)
	retrievedOp1, ok = status.Op("v1")
	if !ok {
		t.Error("expected op1 to still be found")
	}
	if retrievedOp1 == nil {
		t.Error("expected op1 to still be present")
	}
}

func TestOp_NotFound(t *testing.T) {
	status := NewGraphStatus()

	_, ok := status.Op("nonexistent")
	if ok {
		t.Error("expected op not to be found")
	}
}

func TestOps(t *testing.T) {
	status := NewGraphStatus()

	op1 := &mockOperator{name: "op1"}
	op2 := &mockOperator{name: "op2"}

	status.SetOp("v1", op1)
	status.SetOp("v2", op2)

	ops := status.Ops()
	if len(ops) != 2 {
		t.Errorf("expected 2 operators, got %d", len(ops))
	}
	if ops["v1"] == nil {
		t.Error("expected op1 in ops map")
	}
	if ops["v2"] == nil {
		t.Error("expected op2 in ops map")
	}
}

func TestSetFieldValue(t *testing.T) {
	status := NewGraphStatus()

	field1 := &FieldValue{Name: "field1", Value: "value1"}
	field2 := &FieldValue{Name: "field2", Value: 42}

	status.SetFieldValue("field1", field1)
	status.SetFieldValue("field2", field2)

	retrievedField1, ok := status.FieldValue("field1")
	if !ok {
		t.Error("expected field1 to be found")
	}
	if retrievedField1 != field1 {
		t.Errorf("expected field1, got %v", retrievedField1)
	}

	retrievedField2, ok := status.FieldValue("field2")
	if !ok {
		t.Error("expected field2 to be found")
	}
	if retrievedField2 != field2 {
		t.Errorf("expected field2, got %v", retrievedField2)
	}
}

func TestFieldValue_NotFound(t *testing.T) {
	status := NewGraphStatus()

	_, ok := status.FieldValue("nonexistent")
	if ok {
		t.Error("expected field not to be found")
	}
}

func TestSetInDegree(t *testing.T) {
	status := NewGraphStatus()

	v1, _ := graph.NewVertex("v1", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})
	v2, _ := graph.NewVertex("v2", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})

	status.SetInDegree(v1, 3)
	status.SetInDegree(v2, 5)

	if status.InDegree(v1) != 3 {
		t.Errorf("expected in-degree of v1 to be 3, got %d", status.InDegree(v1))
	}
	if status.InDegree(v2) != 5 {
		t.Errorf("expected in-degree of v2 to be 5, got %d", status.InDegree(v2))
	}
}

func TestInDegree_NotFound(t *testing.T) {
	status := NewGraphStatus()

	v, _ := graph.NewVertex("v1", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})

	if status.InDegree(v) != 0 {
		t.Errorf("expected in-degree of unknown vertex to be 0, got %d", status.InDegree(v))
	}
}

func TestAddInDegree(t *testing.T) {
	status := NewGraphStatus()

	v, _ := graph.NewVertex("v1", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})

	status.SetInDegree(v, 5)

	newDegree := status.AddInDegree(v, -2)
	if newDegree != 3 {
		t.Errorf("expected new in-degree to be 3, got %d", newDegree)
	}
	if status.InDegree(v) != 3 {
		t.Errorf("expected in-degree to be 3, got %d", status.InDegree(v))
	}

	newDegree = status.AddInDegree(v, 1)
	if newDegree != 4 {
		t.Errorf("expected new in-degree to be 4, got %d", newDegree)
	}
	if status.InDegree(v) != 4 {
		t.Errorf("expected in-degree to be 4, got %d", status.InDegree(v))
	}
}

func TestAddInDegree_NotFound(t *testing.T) {
	status := NewGraphStatus()

	v, _ := graph.NewVertex("v1", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})

	newDegree := status.AddInDegree(v, -1)
	if newDegree != 0 {
		t.Errorf("expected new in-degree to be 0 for unknown vertex, got %d", newDegree)
	}
}

func TestAddInDegree_Concurrent(t *testing.T) {
	status := NewGraphStatus()

	v, _ := graph.NewVertex("v1", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})

	status.SetInDegree(v, 0)

	var wg sync.WaitGroup
	iterations := 100
	wg.Add(iterations)

	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			status.AddInDegree(v, 1)
		}()
	}
	wg.Wait()

	if status.InDegree(v) != int32(iterations) {
		t.Errorf("expected in-degree to be %d, got %d", iterations, status.InDegree(v))
	}
}

func TestAddStartVertex(t *testing.T) {
	status := NewGraphStatus()

	v1, _ := graph.NewVertex("v1", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})
	v2, _ := graph.NewVertex("v2", &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	})

	status.AddStartVertex(v1)
	status.AddStartVertex(v2)

	startVertices := status.StartVertices()
	if len(startVertices) != 2 {
		t.Errorf("expected 2 start vertices, got %d", len(startVertices))
	}
	if startVertices[0] != v1 {
		t.Error("expected v1 to be first start vertex")
	}
	if startVertices[1] != v2 {
		t.Error("expected v2 to be second start vertex")
	}
}

func TestStartVertices_Empty(t *testing.T) {
	status := NewGraphStatus()

	startVertices := status.StartVertices()
	if len(startVertices) != 0 {
		t.Errorf("expected 0 start vertices, got %d", len(startVertices))
	}
}

func TestNotifyDone(t *testing.T) {
	status := NewGraphStatus()

	// Done channel should not be closed initially
	select {
	case <-status.Done():
		t.Error("expected done channel to be open initially")
	default:
		// Expected
	}

	status.NotifyDone()

	// Done channel should be closed
	select {
	case <-status.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("expected done channel to be closed after NotifyDone")
	}
}

func TestNotifyDone_MultipleCalls(t *testing.T) {
	status := NewGraphStatus()

	status.NotifyDone()
	status.NotifyDone()
	status.NotifyDone()

	// Done channel should only be closed once (no panic)
	select {
	case <-status.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("expected done channel to be closed")
	}
}

func TestSetPendingCount(t *testing.T) {
	status := NewGraphStatus()

	status.SetPendingCount(10)
	if status.PendingCount() != 10 {
		t.Errorf("expected pending count to be 10, got %d", status.PendingCount())
	}

	status.SetPendingCount(5)
	if status.PendingCount() != 5 {
		t.Errorf("expected pending count to be 5, got %d", status.PendingCount())
	}
}

func TestDecreasePendingCount(t *testing.T) {
	status := NewGraphStatus()

	status.SetPendingCount(10)

	newCount := status.DecreasePendingCount()
	if newCount != 9 {
		t.Errorf("expected new pending count to be 9, got %d", newCount)
	}
	if status.PendingCount() != 9 {
		t.Errorf("expected pending count to be 9, got %d", status.PendingCount())
	}

	newCount = status.DecreasePendingCount()
	if newCount != 8 {
		t.Errorf("expected new pending count to be 8, got %d", newCount)
	}
	if status.PendingCount() != 8 {
		t.Errorf("expected pending count to be 8, got %d", status.PendingCount())
	}
}

func TestDecreasePendingCount_Concurrent(t *testing.T) {
	status := NewGraphStatus()

	status.SetPendingCount(100)

	var wg sync.WaitGroup
	iterations := 100
	wg.Add(iterations)

	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			status.DecreasePendingCount()
		}()
	}
	wg.Wait()

	if status.PendingCount() != 0 {
		t.Errorf("expected pending count to be 0, got %d", status.PendingCount())
	}
}

func TestPendingCount_Initial(t *testing.T) {
	status := NewGraphStatus()

	if status.PendingCount() != 0 {
		t.Errorf("expected initial pending count to be 0, got %d", status.PendingCount())
	}
}

func TestWaitGroup(t *testing.T) {
	status := NewGraphStatus()

	status.AddWaitGroup(3)

	// Start goroutines that call DoneWaitGroup
	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			status.DoneWaitGroup()
		}()
	}
	wg.Wait()

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		status.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(1 * time.Second):
		t.Error("expected Wait to complete")
	}
}

func TestWaitGroup_MultipleAdds(t *testing.T) {
	status := NewGraphStatus()

	status.AddWaitGroup(1)
	status.AddWaitGroup(2)
	status.AddWaitGroup(3)

	// Start goroutines that call DoneWaitGroup
	var wg sync.WaitGroup
	wg.Add(6)
	for i := 0; i < 6; i++ {
		go func() {
			defer wg.Done()
			status.DoneWaitGroup()
		}()
	}
	wg.Wait()

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		status.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(1 * time.Second):
		t.Error("expected Wait to complete")
	}
}

func TestWaitGroup_NoGoroutines(t *testing.T) {
	status := NewGraphStatus()

	// Wait should return immediately if no goroutines were added
	done := make(chan struct{})
	go func() {
		status.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("expected Wait to complete immediately")
	}
}
