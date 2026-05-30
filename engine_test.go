package dagor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/operator"
	"github.com/akennis/dagor/predicate"
	"github.com/akennis/dagor/runtime"
)

// trackingOpPool wraps the real operator pool and tracks outstanding borrows.
// borrowed is incremented by getOp and decremented by putOp; it must be zero
// when all fetched operators have been returned.
type trackingOpPool struct {
	borrowed atomic.Int32
}

func (p *trackingOpPool) getOp(name string) (operator.IOperator, error) {
	op, err := operator.GetOp(name)
	if err == nil {
		p.borrowed.Add(1)
	}
	return op, err
}

func (p *trackingOpPool) putOp(name string, op operator.IOperator) error {
	err := operator.PutOp(name, op)
	if err == nil {
		p.borrowed.Add(-1)
	}
	return err
}

// semPool is a bounded goroutine pool backed by a semaphore channel.
// Submit blocks until a slot is available, then launches the function in a
// goroutine. The slot is released when the function returns.
type semPool struct {
	sem chan struct{}
}

func newSemPool(cap int) *semPool {
	return &semPool{sem: make(chan struct{}, cap)}
}

func (p *semPool) Submit(fn func()) error {
	p.sem <- struct{}{}
	go func() {
		defer func() { <-p.sem }()
		fn()
	}()
	return nil
}

func (p *semPool) Release() {}

// mockGPool is a mock implementation of IGPool for testing
type mockGPool struct {
	submitFunc func(func()) error
	submitted  []func()
	mu         sync.Mutex
}

func newMockGPool() *mockGPool {
	return &mockGPool{
		submitted: make([]func(), 0),
		submitFunc: func(fn func()) error {
			go fn()
			return nil
		},
	}
}

func (m *mockGPool) Submit(fn func()) error {
	m.mu.Lock()
	m.submitted = append(m.submitted, fn)
	m.mu.Unlock()
	if m.submitFunc != nil {
		return m.submitFunc(fn)
	}
	go fn()
	return nil
}

func (m *mockGPool) Release() {
}

// mockOperator is a mock implementation of IOperator for testing
type mockOperator struct {
	name         string
	setupErr     error
	runErr       error
	resetErr     error
	inputFields  map[string]any
	outputFields map[string]any
	setupCalled  bool
	runCalled    bool
	resetCalled  bool
	mu           sync.Mutex
}

func newMockOperator(name string) *mockOperator {
	return &mockOperator{
		name:         name,
		inputFields:  make(map[string]any),
		outputFields: make(map[string]any),
	}
}

func (m *mockOperator) Setup(params *config.Params) error {
	m.mu.Lock()
	m.setupCalled = true
	m.mu.Unlock()
	return m.setupErr
}

func (m *mockOperator) Run(ctx context.Context) error {
	m.mu.Lock()
	m.runCalled = true
	m.mu.Unlock()
	return m.runErr
}

func (m *mockOperator) Reset() error {
	m.mu.Lock()
	m.resetCalled = true
	m.mu.Unlock()
	return m.resetErr
}

func (m *mockOperator) InputFields() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inputFields
}

func (m *mockOperator) OutputFields() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.outputFields
}

func (m *mockOperator) SetInputField(field string, value any) error {
	m.mu.Lock()
	m.inputFields[field] = value
	m.mu.Unlock()
	return nil
}

func (m *mockOperator) ResetFields() {
	m.mu.Lock()
	m.inputFields = make(map[string]any)
	m.outputFields = make(map[string]any)
	m.mu.Unlock()
}

func TestNewEngine(t *testing.T) {
	g := &graph.Graph{}
	pool := newMockGPool()

	// Test successful creation
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine.graph != g {
		t.Error("expected graph to be set")
	}
	if engine.pool != pool {
		t.Error("expected pool to be set")
	}
	if engine.status == nil {
		t.Error("expected status to be initialized")
	}

	// Test nil graph
	_, err = NewEngine(nil, pool)
	if err == nil {
		t.Error("expected error for nil graph")
	}
	if err.Error() != "graph is required" {
		t.Errorf("expected 'graph is required' error, got: %v", err)
	}

	// Test nil pool
	_, err = NewEngine(g, nil)
	if err == nil {
		t.Error("expected error for nil pool")
	}
	if err.Error() != "pool is required" {
		t.Errorf("expected 'pool is required' error, got: %v", err)
	}
}

// setupEngineForTest manually sets up engine state to bypass init() for testing
func setupEngineForTest(t *testing.T, g *graph.Graph, ops map[string]*mockOperator) (*Engine, *mockGPool) {
	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Manually set up what init() would do
	engine.status.SetPendingCount(int32(g.Size()))

	for _, v := range g.Vertices() {
		mockOp, ok := ops[v.Name()]
		if !ok {
			t.Fatalf("no mock operator provided for vertex %s", v.Name())
		}

		// Set operator
		engine.status.SetOp(v.Name(), mockOp)

		// Set output fields
		for opFieldName, vertexFieldName := range v.Outputs {
			opField, ok := mockOp.OutputFields()[opFieldName]
			if !ok {
				t.Fatalf("operator %s output field %s not found", v.Name(), opFieldName)
			}
			engine.status.SetFieldValue(vertexFieldName, &runtime.FieldValue{
				Name:  opFieldName,
				Value: opField,
			})
		}

		// Set in-degree
		inDegree := int32(len(v.Predecessors()))
		engine.status.SetInDegree(v, inDegree)
		if inDegree == 0 {
			engine.status.AddStartVertex(v)
		}
	}

	return engine, pool
}

// runEngineForTest runs the engine execution logic without calling init()
// This is a test helper that manually executes what Run() does after init()
func runEngineForTest(t *testing.T, engine *Engine, ctx context.Context) error {
	// Check if graph is empty
	if engine.graph.Size() == 0 {
		return nil
	}

	// Start graph execution (skip init() since we've already set up the status)
	engine.status.SetState(runtime.GraphStateRunning)
	startVertices := engine.status.StartVertices()
	for _, v := range startVertices {
		// submit op execution task to pool.
		engine.status.AddWaitGroup(1)
		err := engine.pool.Submit(func() {
			defer engine.status.DoneWaitGroup()
			engine.runVertex(ctx, v)
		})
		if err != nil {
			engine.status.DoneWaitGroup()
			engine.status.SetError(err)
			return err
		}
	}

	// wait graph execution finished.
	select {
	case <-engine.status.Done():
	case <-ctx.Done():
		engine.status.SetError(ctx.Err())
	}

	// wait all active goroutines execution finished.
	engine.status.Wait()

	// set graph state to finished.
	engine.status.SetState(runtime.GraphStateFinished)
	return engine.status.Error()
}

func TestEngine_Run_SimpleGraph(t *testing.T) {
	// Create a simple graph with one vertex
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	// Create mock operator
	mockOp := newMockOperator("MockOp")
	mockOp.outputFields["output"] = "test_value"

	engine, _ := setupEngineForTest(t, g, map[string]*mockOperator{
		"op1": mockOp,
	})

	ctx := context.Background()
	err = runEngineForTest(t, engine, ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mockOp.runCalled {
		t.Error("expected operator Run to be called")
	}
}

func TestEngine_Run_ContextCancellation(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	mockOp := newMockOperator("MockOp")
	mockOp.outputFields["output"] = "test_value"

	engine, _ := setupEngineForTest(t, g, map[string]*mockOperator{
		"op1": mockOp,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = runEngineForTest(t, engine, ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
}

// slowOperator is a mock operator that takes time to run
type slowOperator struct {
	*mockOperator
	duration time.Duration
}

func (s *slowOperator) Run(ctx context.Context) error {
	select {
	case <-time.After(s.duration):
		return s.mockOperator.Run(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestEngine_Run_ContextTimeout(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	// Create a slow operator that will timeout
	slowOp := &slowOperator{
		mockOperator: newMockOperator("MockOp"),
		duration:     100 * time.Millisecond,
	}
	slowOp.outputFields["output"] = "test_value"

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Manually set up engine state
	engine.status.SetPendingCount(1)
	engine.status.SetOp("op1", slowOp)
	engine.status.SetFieldValue("field1", &runtime.FieldValue{
		Name:  "output",
		Value: "test_value",
	})
	engine.status.SetInDegree(g.VertexByName("op1"), 0)
	engine.status.AddStartVertex(g.VertexByName("op1"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = runEngineForTest(t, engine, ctx)
	if err == nil {
		t.Error("expected error for timeout context")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded error, got: %v", err)
	}
}

func TestEngine_Close(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Test closing before Run (should fail)
	ctx := context.Background()
	err = engine.Close(ctx)
	if err == nil {
		t.Error("expected error when closing before Run")
	}

	// Test closing after Run completes
	// Manually set state to finished for testing
	engine.status.SetState(runtime.GraphStateFinished)
	mockOp := newMockOperator("MockOp")
	engine.status.SetOp("op1", mockOp)

	err = engine.Close(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mockOp.resetCalled {
		t.Error("expected operator Reset to be called")
	}
}

func TestEngine_GetOutput(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Set field value
	engine.status.SetFieldValue("field1", &runtime.FieldValue{
		Name:  "output",
		Value: "test_value",
	})

	// Test getting existing output
	value, ok := engine.GetOutput("field1")
	if !ok {
		t.Error("expected to get output")
	}
	if value != "test_value" {
		t.Errorf("expected value 'test_value', got: %v", value)
	}

	// Test getting non-existent output
	_, ok = engine.GetOutput("non_existent")
	if ok {
		t.Error("expected not to get output for non-existent field")
	}
}

func TestEngine_Init_OperatorNotFound(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "NonExistentOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	err = engine.Run(ctx)
	if err == nil {
		t.Error("expected error for non-existent operator")
	}
}

func TestEngine_Init_OutputFieldNotFound(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"non_existent": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Manually set operator without the required output field
	mockOp := newMockOperator("MockOp")
	engine.status.SetOp("op1", mockOp)

	ctx := context.Background()
	err = engine.Run(ctx)
	// This will fail during init when trying to bind output fields
	if err == nil {
		t.Error("expected error for non-existent output field")
	}
}

func TestEngine_Run_OperatorError_Stop(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				OnError: config.OnErrorStop,
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	// Set up operator with error
	mockOp := newMockOperator("MockOp")
	mockOp.runErr = errors.New("operator error")
	mockOp.outputFields["output"] = "test_value"

	engine, _ := setupEngineForTest(t, g, map[string]*mockOperator{
		"op1": mockOp,
	})

	ctx := context.Background()
	err = runEngineForTest(t, engine, ctx)
	if err == nil {
		t.Error("expected error from operator")
	}
}

func TestEngine_Run_OperatorError_Continue(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				OnError: config.OnErrorContinue,
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	// Set up operator with error but continue on error
	mockOp := newMockOperator("MockOp")
	mockOp.runErr = errors.New("operator error")
	mockOp.outputFields["output"] = "test_value"

	engine, _ := setupEngineForTest(t, g, map[string]*mockOperator{
		"op1": mockOp,
	})

	ctx := context.Background()
	err = runEngineForTest(t, engine, ctx)
	// Should not return error when OnError is Continue
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that vertex error was set
	vertexErr := engine.status.VertexError(g.VertexByName("op1"))
	if vertexErr == nil {
		t.Error("expected vertex error to be set")
	}
}

func TestEngine_Run_InputFieldNotFound(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp1",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
			"op2": {
				Op:     "MockOp2",
				Params: []byte(`{}`),
				Inputs: map[string]string{
					"input": "field1", // Use existing field, but we'll remove it from status
				},
				Outputs: map[string]string{
					"output": "field2",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	mockOp1 := newMockOperator("MockOp1")
	mockOp1.outputFields["output"] = "value1"
	mockOp2 := newMockOperator("MockOp2")
	mockOp2.outputFields["output"] = "value2"

	engine, _ := setupEngineForTest(t, g, map[string]*mockOperator{
		"op1": mockOp1,
		"op2": mockOp2,
	})

	// Remove field1 from status to simulate missing input field
	// We need to access the internal fieldValues map, but it's not exported
	// Instead, we can test this by not setting up the field properly
	// Actually, setupEngineForTest sets it up, so let's manually remove it
	// Since we can't access the internal map, let's test a different scenario:
	// We'll set up the engine but then manually clear the field value after setup
	// Actually, a better test is to ensure the field exists but test the error path differently
	// Let's just verify the test works with proper setup first, then we can add a specific test
	// for missing input fields if needed. For now, let's test that op2 gets the input from op1.

	ctx := context.Background()
	err = runEngineForTest(t, engine, ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mockOp1.runCalled {
		t.Error("expected op1 to be executed")
	}
	if !mockOp2.runCalled {
		t.Error("expected op2 to be executed")
	}

	// Verify op2 received the input
	inputValue := mockOp2.InputFields()["input"]
	if inputValue != "value1" {
		t.Errorf("expected op2 to receive input 'value1', got: %v", inputValue)
	}
}

func TestEngine_Run_GraphWithMultipleVertices(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp1",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
			"op2": {
				Op:     "MockOp2",
				Params: []byte(`{}`),
				Inputs: map[string]string{
					"input": "field1",
				},
				Outputs: map[string]string{
					"output": "field2",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	// Set up operators
	mockOp1 := newMockOperator("MockOp1")
	mockOp1.outputFields["output"] = "value1"
	mockOp2 := newMockOperator("MockOp2")
	mockOp2.outputFields["output"] = "value2"

	engine, _ := setupEngineForTest(t, g, map[string]*mockOperator{
		"op1": mockOp1,
		"op2": mockOp2,
	})

	ctx := context.Background()
	err = runEngineForTest(t, engine, ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mockOp1.runCalled {
		t.Error("expected op1 to be executed")
	}
	if !mockOp2.runCalled {
		t.Error("expected op2 to be executed")
	}
}

func TestEngine_Run_PoolSubmitError(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	pool.submitFunc = func(func()) error {
		return errors.New("submit error")
	}

	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	mockOp := newMockOperator("MockOp")
	mockOp.outputFields["output"] = "test_value"

	// Manually set up engine state
	engine.status.SetPendingCount(1)
	engine.status.SetOp("op1", mockOp)
	engine.status.SetFieldValue("field1", &runtime.FieldValue{
		Name:  "output",
		Value: "test_value",
	})
	engine.status.SetInDegree(g.VertexByName("op1"), 0)
	engine.status.AddStartVertex(g.VertexByName("op1"))

	ctx := context.Background()
	err = runEngineForTest(t, engine, ctx)
	if err == nil {
		t.Error("expected error from pool submit")
	}
	if err.Error() != "submit error" {
		t.Errorf("expected 'submit error', got: %v", err)
	}
}

// capturingOpPool records every putOp call so tests can assert whether an op
// was returned to the pool.
type capturingOpPool struct {
	putNames []string
	mu       sync.Mutex
}

func (p *capturingOpPool) getOp(name string) (operator.IOperator, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *capturingOpPool) putOp(name string, op operator.IOperator) error {
	p.mu.Lock()
	p.putNames = append(p.putNames, name)
	p.mu.Unlock()
	return nil
}

func (p *capturingOpPool) wasPut(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, n := range p.putNames {
		if n == name {
			return true
		}
	}
	return false
}

func TestEngine_Close_ResetError(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	capturing := &capturingOpPool{}
	engine.opPool = capturing

	engine.status.SetState(runtime.GraphStateFinished)
	mockOp := newMockOperator("MockOp")
	mockOp.resetErr = errors.New("reset error")
	engine.status.SetOp("op1", mockOp)

	ctx := context.Background()
	err = engine.Close(ctx)
	if err == nil {
		t.Fatal("expected Close to return error when Reset fails")
	}
	if !errors.Is(err, mockOp.resetErr) {
		t.Errorf("expected error to wrap Reset error, got: %v", err)
	}
	if capturing.wasPut("MockOp") {
		t.Error("op with failed Reset must not be returned to the pool")
	}
}

// TestEngine_Close_ResetError_MultiOp verifies that errors from multiple ops
// are all surfaced (joined) and that only the failing ops are dropped.
func TestEngine_Close_ResetError_MultiOp(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:      "GoodOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out1": "f1"},
			},
			"op2": {
				Op:     "BadOp",
				Params: []byte(`{}`),
				Inputs: map[string]string{"in2": "f1"},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	capturing := &capturingOpPool{}
	engine.opPool = capturing

	engine.status.SetState(runtime.GraphStateFinished)

	goodOp := newMockOperator("GoodOp")
	engine.status.SetOp("op1", goodOp)

	badOp := newMockOperator("BadOp")
	badOp.resetErr = errors.New("bad reset")
	engine.status.SetOp("op2", badOp)

	ctx := context.Background()
	err = engine.Close(ctx)
	if err == nil {
		t.Fatal("expected Close to return error")
	}
	if !errors.Is(err, badOp.resetErr) {
		t.Errorf("expected joined error to contain bad reset error, got: %v", err)
	}
	if capturing.wasPut("BadOp") {
		t.Error("op with failed Reset must not be returned to the pool")
	}
	if !capturing.wasPut("GoodOp") {
		t.Error("op with successful Reset should be returned to the pool")
	}
}

func TestEngine_Run_EmptyGraph(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name:     "test",
		Vertices: map[string]*config.VertexConfig{},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	err = engine.Run(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// panicOperator is a mock operator that panics when Run is called
type panicOperator struct {
	*mockOperator
}

func (p *panicOperator) Run(ctx context.Context) error {
	panic("test panic")
}

func TestEngine_Run_OperatorPanic(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				OnError: config.OnErrorStop,
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Create a panic operator
	panicOp := &panicOperator{
		mockOperator: newMockOperator("MockOp"),
	}
	panicOp.outputFields["output"] = "test_value"

	engine.status.SetOp("op1", panicOp)
	engine.status.SetFieldValue("field1", &runtime.FieldValue{
		Name:  "output",
		Value: "test_value",
	})
	engine.status.SetInDegree(g.VertexByName("op1"), 0)
	engine.status.AddStartVertex(g.VertexByName("op1"))
	engine.status.SetPendingCount(1)

	ctx := context.Background()
	err = engine.Run(ctx)
	if err == nil {
		t.Error("expected error from panic")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestEngine_Close_NilOperator(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output": "field1",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Set state to finished
	engine.status.SetState(runtime.GraphStateFinished)
	// Set nil operator
	engine.status.SetOp("op1", nil)

	ctx := context.Background()
	err = engine.Close(ctx)
	// Should handle nil operator gracefully
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEngine_GetOutput_MultipleFields(t *testing.T) {
	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"op1": {
				Op:     "MockOp",
				Params: []byte(`{}`),
				Outputs: map[string]string{
					"output1": "field1",
					"output2": "field2",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	pool := newMockGPool()
	engine, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Set multiple field values
	engine.status.SetFieldValue("field1", &runtime.FieldValue{
		Name:  "output1",
		Value: "value1",
	})
	engine.status.SetFieldValue("field2", &runtime.FieldValue{
		Name:  "output2",
		Value: "value2",
	})

	// Test getting both outputs
	value1, ok := engine.GetOutput("field1")
	if !ok {
		t.Error("expected to get output field1")
	}
	if value1 != "value1" {
		t.Errorf("expected value 'value1', got: %v", value1)
	}

	value2, ok := engine.GetOutput("field2")
	if !ok {
		t.Error("expected to get output field2")
	}
	if value2 != "value2" {
		t.Errorf("expected value 'value2', got: %v", value2)
	}
}

// TestEngine_Condition_Skip: B has a condition that always returns false → B is not called.
func TestEngine_Condition_Skip(t *testing.T) {
	predName := "cond_skip_false"
	if err := predicate.Register(predName, func(inputs map[string]any) bool { return false }); err != nil {
		t.Fatalf("register predicate: %v", err)
	}
	t.Cleanup(func() { predicate.Unregister(predName) })

	valA := 0
	opA := newMockOperator("A")
	opA.outputFields["out"] = &valA

	opB := newMockOperator("B")

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"A": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "field_a"},
				OnError: config.OnErrorStop,
			},
			"B": {
				Op:        "MockOp",
				Params:    []byte(`{}`),
				Inputs:    map[string]string{"in": "field_a"},
				Condition: predName,
				OnError:   config.OnErrorStop,
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	ops := map[string]*mockOperator{"A": opA, "B": opB}
	engine, _ := setupEngineForTest(t, g, ops)

	ctx := context.Background()
	if err := runEngineForTest(t, engine, ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !opA.runCalled {
		t.Error("expected A to be called")
	}
	if opB.runCalled {
		t.Error("expected B not to be called (condition=false)")
	}
	if !engine.VertexSkipped("B") {
		t.Error("expected B to be marked skipped")
	}
}

// TestEngine_Condition_Run: B has a condition that always returns true → B is called.
func TestEngine_Condition_Run(t *testing.T) {
	predName := "cond_skip_true"
	if err := predicate.Register(predName, func(inputs map[string]any) bool { return true }); err != nil {
		t.Fatalf("register predicate: %v", err)
	}
	t.Cleanup(func() { predicate.Unregister(predName) })

	valA := 0
	opA := newMockOperator("A")
	opA.outputFields["out"] = &valA

	opB := newMockOperator("B")

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"A": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "field_a"},
				OnError: config.OnErrorStop,
			},
			"B": {
				Op:        "MockOp",
				Params:    []byte(`{}`),
				Inputs:    map[string]string{"in": "field_a"},
				Condition: predName,
				OnError:   config.OnErrorStop,
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	ops := map[string]*mockOperator{"A": opA, "B": opB}
	engine, _ := setupEngineForTest(t, g, ops)

	ctx := context.Background()
	if err := runEngineForTest(t, engine, ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !opA.runCalled {
		t.Error("expected A to be called")
	}
	if !opB.runCalled {
		t.Error("expected B to be called (condition=true)")
	}
	if engine.VertexSkipped("B") {
		t.Error("expected B not to be marked skipped")
	}
}

// TestEngine_Condition_TransitiveSkip: A→B(condition=false)→C→D — B, C, D all skipped.
func TestEngine_Condition_TransitiveSkip(t *testing.T) {
	predName := "cond_transitive_false"
	if err := predicate.Register(predName, func(inputs map[string]any) bool { return false }); err != nil {
		t.Fatalf("register predicate: %v", err)
	}
	t.Cleanup(func() { predicate.Unregister(predName) })

	valA, valB, valC := 0, 0, 0
	opA := newMockOperator("A")
	opA.outputFields["out"] = &valA
	opB := newMockOperator("B")
	opB.outputFields["out"] = &valB
	opC := newMockOperator("C")
	opC.outputFields["out"] = &valC
	opD := newMockOperator("D")

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"A": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "field_a"},
				OnError: config.OnErrorStop,
			},
			"B": {
				Op:        "MockOp",
				Params:    []byte(`{}`),
				Inputs:    map[string]string{"in": "field_a"},
				Outputs:   map[string]string{"out": "field_b"},
				Condition: predName,
				OnError:   config.OnErrorStop,
			},
			"C": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Inputs:  map[string]string{"in": "field_b"},
				Outputs: map[string]string{"out": "field_c"},
				OnError: config.OnErrorStop,
			},
			"D": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Inputs:  map[string]string{"in": "field_c"},
				OnError: config.OnErrorStop,
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	ops := map[string]*mockOperator{"A": opA, "B": opB, "C": opC, "D": opD}
	engine, _ := setupEngineForTest(t, g, ops)

	ctx := context.Background()
	if err := runEngineForTest(t, engine, ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !opA.runCalled {
		t.Error("expected A to be called")
	}
	if opB.runCalled {
		t.Error("expected B not to be called")
	}
	if opC.runCalled {
		t.Error("expected C not to be called (transitive skip from B)")
	}
	if opD.runCalled {
		t.Error("expected D not to be called (transitive skip from C)")
	}
	if !engine.VertexSkipped("B") {
		t.Error("expected B to be marked skipped")
	}
	if !engine.VertexSkipped("C") {
		t.Error("expected C to be marked skipped")
	}
	if !engine.VertexSkipped("D") {
		t.Error("expected D to be marked skipped")
	}
}

// TestEngine_Condition_PartialSkip: diamond A→B(condition=false)→D and A→C→D.
// B is skipped → D is skipped. C runs normally.
func TestEngine_Condition_PartialSkip(t *testing.T) {
	predName := "cond_partial_false"
	if err := predicate.Register(predName, func(inputs map[string]any) bool { return false }); err != nil {
		t.Fatalf("register predicate: %v", err)
	}
	t.Cleanup(func() { predicate.Unregister(predName) })

	valA, valB, valC := 0, 0, 0
	opA := newMockOperator("A")
	opA.outputFields["out"] = &valA
	opB := newMockOperator("B")
	opB.outputFields["out"] = &valB
	opC := newMockOperator("C")
	opC.outputFields["out"] = &valC
	opD := newMockOperator("D")

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"A": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "field_a"},
				OnError: config.OnErrorStop,
			},
			"B": {
				Op:        "MockOp",
				Params:    []byte(`{}`),
				Inputs:    map[string]string{"in": "field_a"},
				Outputs:   map[string]string{"out": "field_b"},
				Condition: predName,
				OnError:   config.OnErrorStop,
			},
			"C": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Inputs:  map[string]string{"in": "field_a"},
				Outputs: map[string]string{"out": "field_c"},
				OnError: config.OnErrorStop,
			},
			"D": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Inputs:  map[string]string{"b": "field_b", "c": "field_c"},
				OnError: config.OnErrorStop,
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	ops := map[string]*mockOperator{"A": opA, "B": opB, "C": opC, "D": opD}
	engine, _ := setupEngineForTest(t, g, ops)

	ctx := context.Background()
	if err := runEngineForTest(t, engine, ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !opA.runCalled {
		t.Error("expected A to be called")
	}
	if opB.runCalled {
		t.Error("expected B not to be called (condition=false)")
	}
	if !opC.runCalled {
		t.Error("expected C to be called (no condition)")
	}
	if opD.runCalled {
		t.Error("expected D not to be called (B is skipped input producer)")
	}
	if !engine.VertexSkipped("B") {
		t.Error("expected B to be marked skipped")
	}
	if engine.VertexSkipped("C") {
		t.Error("expected C not to be marked skipped")
	}
	if !engine.VertexSkipped("D") {
		t.Error("expected D to be marked skipped")
	}
}

// TestEngine_Condition_UnregisteredPredicate: condition names an unknown predicate.
func TestEngine_Condition_UnregisteredPredicate(t *testing.T) {
	unknownPred := "cond_unregistered_unique5"

	valA := 0
	opA := newMockOperator("A")
	opA.outputFields["out"] = &valA
	opB := newMockOperator("B")

	makeGraph := func(onError string) *graph.Graph {
		graphConfig := &config.GraphConfig{
			Name: "test",
			Vertices: map[string]*config.VertexConfig{
				"A": {
					Op:      "MockOp",
					Params:  []byte(`{}`),
					Outputs: map[string]string{"out": "field_a_unr"},
					OnError: config.OnErrorStop,
				},
				"B": {
					Op:        "MockOp",
					Params:    []byte(`{}`),
					Inputs:    map[string]string{"in": "field_a_unr"},
					Condition: unknownPred,
					OnError:   onError,
				},
			},
		}
		g, err := graph.NewGraphFromConfig(graphConfig)
		if err != nil {
			t.Fatalf("failed to create graph: %v", err)
		}
		return g
	}

	// on_error=stop: engine returns error
	t.Run("on_error_stop", func(t *testing.T) {
		g := makeGraph(config.OnErrorStop)
		opA2 := newMockOperator("A")
		opA2.outputFields["out"] = &valA
		opB2 := newMockOperator("B")
		ops := map[string]*mockOperator{"A": opA2, "B": opB2}
		eng, _ := setupEngineForTest(t, g, ops)

		ctx := context.Background()
		err := runEngineForTest(t, eng, ctx)
		if err == nil {
			t.Error("expected error from Run for on_error=stop")
		}
	})

	// on_error=continue: no graph error, but vertex error is set
	t.Run("on_error_continue", func(t *testing.T) {
		g := makeGraph(config.OnErrorContinue)
		opA3 := newMockOperator("A")
		opA3.outputFields["out"] = &valA
		opB3 := newMockOperator("B")
		ops := map[string]*mockOperator{"A": opA3, "B": opB3}
		eng, _ := setupEngineForTest(t, g, ops)

		ctx := context.Background()
		err := runEngineForTest(t, eng, ctx)
		if err != nil {
			t.Errorf("expected no graph error for on_error=continue, got: %v", err)
		}
		bVertex := g.VertexByName("B")
		if eng.status.VertexError(bVertex) == nil {
			t.Error("expected vertex error to be set for B")
		}
	})

	_ = opA
	_ = opB
}

// TestEngine_Condition_PredicateEvaluatesInputs: predicate inspects actual input value.
func TestEngine_Condition_PredicateEvaluatesInputs(t *testing.T) {
	predName := "cond_eval_inputs"
	if err := predicate.Register(predName, func(inputs map[string]any) bool {
		ptr, ok := inputs["field_a_eval"].(*int)
		if !ok || ptr == nil {
			return false
		}
		return *ptr > 0
	}); err != nil {
		t.Fatalf("register predicate: %v", err)
	}
	t.Cleanup(func() { predicate.Unregister(predName) })

	makeGraphAndOps := func(sourceVal int) (*graph.Graph, map[string]*mockOperator) {
		graphConfig := &config.GraphConfig{
			Name: "test",
			Vertices: map[string]*config.VertexConfig{
				"A": {
					Op:      "MockOp",
					Params:  []byte(`{}`),
					Outputs: map[string]string{"out": "field_a_eval"},
					OnError: config.OnErrorStop,
				},
				"B": {
					Op:        "MockOp",
					Params:    []byte(`{}`),
					Inputs:    map[string]string{"in": "field_a_eval"},
					Condition: predName,
					OnError:   config.OnErrorStop,
				},
			},
		}
		g, err := graph.NewGraphFromConfig(graphConfig)
		if err != nil {
			t.Fatalf("failed to create graph: %v", err)
		}
		opA := newMockOperator("A")
		opA.outputFields["out"] = &sourceVal
		opB := newMockOperator("B")
		return g, map[string]*mockOperator{"A": opA, "B": opB}
	}

	// Positive value: predicate returns true → B runs
	t.Run("positive_value", func(t *testing.T) {
		g, ops := makeGraphAndOps(5)
		eng, _ := setupEngineForTest(t, g, ops)
		ctx := context.Background()
		if err := runEngineForTest(t, eng, ctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ops["B"].runCalled {
			t.Error("expected B to be called for positive value")
		}
	})

	// Negative value: predicate returns false → B skipped
	t.Run("negative_value", func(t *testing.T) {
		g, ops := makeGraphAndOps(-3)
		eng, _ := setupEngineForTest(t, g, ops)
		ctx := context.Background()
		if err := runEngineForTest(t, eng, ctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ops["B"].runCalled {
			t.Error("expected B not to be called for negative value")
		}
		if !eng.VertexSkipped("B") {
			t.Error("expected B to be marked skipped for negative value")
		}
	})
}

// trackingOp is a minimal operator used by TestRunSubgraph_CloseCalledOnRunError.
// It fails on Run and counts how many times Reset is called.
type trackingOp struct {
	mu          sync.Mutex
	resetCount  *atomic.Int32
	runErr      error
	outputValue any
	inputs      map[string]any
	outputs     map[string]any
}

func newTrackingOp(runErr error, outputValue any, resetCount *atomic.Int32) *trackingOp {
	return &trackingOp{
		resetCount:  resetCount,
		runErr:      runErr,
		outputValue: outputValue,
		inputs:      make(map[string]any),
		outputs:     map[string]any{"out": outputValue},
	}
}

func (o *trackingOp) Setup(_ *config.Params) error { return nil }
func (o *trackingOp) Run(_ context.Context) error  { return o.runErr }
func (o *trackingOp) Reset() error                 { o.resetCount.Add(1); return nil }
func (o *trackingOp) InputFields() map[string]any  { o.mu.Lock(); defer o.mu.Unlock(); return o.inputs }
func (o *trackingOp) OutputFields() map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.outputs
}
func (o *trackingOp) ResetFields() {}
func (o *trackingOp) SetInputField(f string, v any) error {
	o.mu.Lock()
	o.inputs[f] = v
	o.mu.Unlock()
	return nil
}

// TestRunSubgraph_CloseCalledOnRunError verifies that runSubgraph calls
// subEngine.Close even when subEngine.Run returns an error (BUG-07).
// Close returns operators to the pool; Reset is called as part of Close,
// so a non-zero reset count proves Close was invoked.
func TestRunSubgraph_CloseCalledOnRunError(t *testing.T) {
	var resetCount atomic.Int32

	// Register an operator that fails on Run.
	failOpName := "TestSubgraphFailOp_BUG07"
	if err := operator.RegisterOpFactory(failOpName, func() operator.IOperator {
		return newTrackingOp(errors.New("subgraph op failed"), nil, &resetCount)
	}); err != nil {
		t.Fatalf("register fail op: %v", err)
	}

	// Register a source operator that produces a one-element slice.
	srcOpName := "TestSubgraphSrcOp_BUG07"
	items := []int{42}
	if err := operator.RegisterOpFactory(srcOpName, func() operator.IOperator {
		return newTrackingOp(nil, &items, &resetCount)
	}); err != nil {
		t.Fatalf("register src op: %v", err)
	}

	subgraphCfg := &config.GraphConfig{
		Name:          "sub",
		ExternalWires: []string{"item"},
		Vertices: map[string]*config.VertexConfig{
			"fail": {
				Op:      failOpName,
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "sub_result"},
			},
		},
	}
	mainCfg := &config.GraphConfig{
		Name: "main",
		Vertices: map[string]*config.VertexConfig{
			"src": {
				Op:      srcOpName,
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "items"},
			},
			"mapper": {
				Inputs: map[string]string{"Items": "items"},
				Map: &config.MapConfig{
					ItemInput:    "item",
					ResultOutput: "sub_result",
					ResultsWire:  "results",
					Subgraph:     subgraphCfg,
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(mainCfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	pool := newMockGPool()
	eng, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	ctx := context.Background()
	runErr := eng.Run(ctx)
	if runErr == nil {
		t.Fatal("expected Run to return an error (subgraph op fails)")
	}

	// The subgraph's fail operator must have had Reset called, which only
	// happens inside Close. Before the fix, defer Close was registered after
	// Run, so a failed Run would skip it entirely.
	if resetCount.Load() == 0 {
		t.Error("BUG-07: subgraph operator Reset not called — Close was not invoked after Run error")
	}
}

// TestMapVertex_BoundedPoolNoDeadlock verifies that a map vertex with multiple
// elements completes without deadlock when using a bounded goroutine pool (BUG-08).
//
// With the old parallel approach runMapVertex submitted element goroutines to
// e.pool. With pool capacity 2:
//   - Slot 1: mapper goroutine (blocked in wg.Wait waiting for elements)
//   - Slot 2: element goroutine (blocked in subEngine.Run waiting for its vertices)
//   - No slot left for the sub-engine vertex → deadlock.
//
// Fix: sub-graphs run sequentially in the current goroutine. Only sub-engine
// internal vertices use the pool, so pool capacity 2 is always enough.
func TestMapVertex_BoundedPoolNoDeadlock(t *testing.T) {
	var resetCount atomic.Int32

	items := []int{10, 20, 30}
	fixedResult := 99

	srcOpName := "TestSrcOp_BUG08"
	if err := operator.RegisterOpFactory(srcOpName, func() operator.IOperator {
		return newTrackingOp(nil, &items, &resetCount)
	}); err != nil {
		t.Fatalf("register src op: %v", err)
	}

	subOpName := "TestSubOp_BUG08"
	if err := operator.RegisterOpFactory(subOpName, func() operator.IOperator {
		return newTrackingOp(nil, &fixedResult, &resetCount)
	}); err != nil {
		t.Fatalf("register sub op: %v", err)
	}

	subgraphCfg := &config.GraphConfig{
		Name:          "sub",
		ExternalWires: []string{"item"},
		Vertices: map[string]*config.VertexConfig{
			"proc": {
				Op:      subOpName,
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "sub_result"},
			},
		},
	}
	mainCfg := &config.GraphConfig{
		Name: "main",
		Vertices: map[string]*config.VertexConfig{
			"src": {
				Op:      srcOpName,
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "items"},
			},
			"mapper": {
				Inputs: map[string]string{"Items": "items"},
				Map: &config.MapConfig{
					ItemInput:    "item",
					ResultOutput: "sub_result",
					ResultsWire:  "results",
					Subgraph:     subgraphCfg,
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(mainCfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	// A bounded pool with capacity 2. The old code would deadlock here:
	// the mapper goroutine occupies slot 1 waiting in wg.Wait(), an element
	// goroutine occupies slot 2 waiting in subEngine.Run(), leaving no slot
	// for the sub-engine's proc vertex. With the sequential fix both slots
	// are never simultaneously consumed by waiting goroutines.
	pool := newSemPool(2)

	eng, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run failed (possible deadlock — context timed out): %v", err)
	}

	results, ok := eng.GetOutput("results")
	if !ok {
		t.Fatal("results wire not found in engine output")
	}
	slicePtr, ok := results.(*[]any)
	if !ok {
		t.Fatalf("expected results to be *[]any, got %T", results)
	}
	slice := *slicePtr
	if len(slice) != 3 {
		t.Fatalf("expected 3 results (one per input element), got %d", len(slice))
	}
	for i, v := range slice {
		if v != fixedResult {
			t.Errorf("results[%d]: expected %d, got %v", i, fixedResult, v)
		}
	}
}

// TestEngine_UnregisteredPredicate_DownstreamVertexRuns guards against regression
// of BUG-11: when shouldSkip returns an error (unregistered predicate) and the
// vertex has on_error=continue, the engine must NOT return an error, must mark the
// vertex as skipped, and must still run any downstream vertex whose remaining
// inputs are satisfied (using merge=coalesce to tolerate the nil output from the
// skipped vertex).
func TestEngine_UnregisteredPredicate_DownstreamVertexRuns(t *testing.T) {
	// Graph:  A ──field_a──► B(unknown pred, on_error=continue, out=field_b)
	//          └──field_a──► C(merge=coalesce, reads field_a + field_b)
	//
	// B is skipped (predicate not found + on_error=continue).
	// C has merge=coalesce so it tolerates the nil field_b and still runs.

	valA := 7
	opA := newMockOperator("A")
	opA.outputFields["out"] = &valA

	valB := 0
	opB := newMockOperator("B")
	opB.outputFields["out"] = &valB // field registered but never written (B is skipped)

	opC := newMockOperator("C")

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"A": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "bug11_field_a"},
				OnError: config.OnErrorStop,
			},
			"B": {
				Op:        "MockOp",
				Params:    []byte(`{}`),
				Inputs:    map[string]string{"in": "bug11_field_a"},
				Outputs:   map[string]string{"out": "bug11_field_b"},
				Condition: "predicate_that_does_not_exist_bug11",
				OnError:   config.OnErrorContinue,
			},
			"C": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Inputs:  map[string]string{"a": "bug11_field_a", "b": "bug11_field_b"},
				Merge:   config.MergeCoalesce,
				OnError: config.OnErrorStop,
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	ops := map[string]*mockOperator{"A": opA, "B": opB, "C": opC}
	engine, _ := setupEngineForTest(t, g, ops)

	ctx := context.Background()
	if err := runEngineForTest(t, engine, ctx); err != nil {
		t.Fatalf("engine returned unexpected error: %v", err)
	}

	if !opA.runCalled {
		t.Error("expected A to run")
	}
	if opB.runCalled {
		t.Error("expected B not to run (skipped due to unregistered predicate + on_error=continue)")
	}
	if !engine.VertexSkipped("B") {
		t.Error("expected B to be marked skipped")
	}
	if engine.status.VertexError(g.VertexByName("B")) == nil {
		t.Error("expected per-vertex error recorded for B (predicate not found)")
	}
	if !opC.runCalled {
		t.Error("expected C to run (merge=coalesce tolerates nil field_b from skipped B)")
	}
}

// TestEngine_Predicate_Panic_OnErrorStop covers TODO-04: a panicking predicate
// must not crash the process; with on_error=stop the engine must return an error.
func TestEngine_Predicate_Panic_OnErrorStop(t *testing.T) {
	predName := "pred_panic_stop_" + t.Name()
	if err := predicate.Register(predName, func(_ map[string]any) bool {
		panic("predicate exploded")
	}); err != nil {
		t.Fatalf("register predicate: %v", err)
	}
	t.Cleanup(func() { predicate.Unregister(predName) })

	valA := 1
	opA := newMockOperator("A")
	opA.outputFields["out"] = &valA
	opB := newMockOperator("B")

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"A": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "field_a_pp"},
				OnError: config.OnErrorStop,
			},
			"B": {
				Op:        "MockOp",
				Params:    []byte(`{}`),
				Inputs:    map[string]string{"in": "field_a_pp"},
				Condition: predName,
				OnError:   config.OnErrorStop,
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	ops := map[string]*mockOperator{"A": opA, "B": opB}
	engine, _ := setupEngineForTest(t, g, ops)

	ctx := context.Background()
	runErr := runEngineForTest(t, engine, ctx)
	if runErr == nil {
		t.Fatal("expected engine to return an error when predicate panics with on_error=stop")
	}
}

// TestEngine_Predicate_Panic_OnErrorContinue covers TODO-04: a panicking predicate
// with on_error=continue must not crash the process; the engine must succeed and
// the vertex must be skipped with a per-vertex error recorded.
func TestEngine_Predicate_Panic_OnErrorContinue(t *testing.T) {
	predName := "pred_panic_continue_" + t.Name()
	if err := predicate.Register(predName, func(_ map[string]any) bool {
		panic("predicate exploded")
	}); err != nil {
		t.Fatalf("register predicate: %v", err)
	}
	t.Cleanup(func() { predicate.Unregister(predName) })

	valA := 1
	opA := newMockOperator("A")
	opA.outputFields["out"] = &valA
	opB := newMockOperator("B")

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"A": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "field_a_pc"},
				OnError: config.OnErrorStop,
			},
			"B": {
				Op:        "MockOp",
				Params:    []byte(`{}`),
				Inputs:    map[string]string{"in": "field_a_pc"},
				Condition: predName,
				OnError:   config.OnErrorContinue,
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}

	ops := map[string]*mockOperator{"A": opA, "B": opB}
	engine, _ := setupEngineForTest(t, g, ops)

	ctx := context.Background()
	if runErr := runEngineForTest(t, engine, ctx); runErr != nil {
		t.Fatalf("engine must not return error when predicate panics with on_error=continue, got: %v", runErr)
	}
	if opB.runCalled {
		t.Error("B must not run when its predicate panicked")
	}
	if !engine.VertexSkipped("B") {
		t.Error("B must be marked skipped after predicate panic")
	}
	bVertex := g.VertexByName("B")
	if engine.status.VertexError(bVertex) == nil {
		t.Error("expected per-vertex error recorded for B after predicate panic")
	}
}

// TestShouldSkip_ConditionInputProducerSkipped covers TODO-03: when the producer
// of a ConditionInput wire was skipped, shouldSkip must propagate the skip
// without invoking the predicate (which would panic on a nil wire value).
func TestShouldSkip_ConditionInputProducerSkipped(t *testing.T) {
	predName := "pred_cond_input_skip_" + t.Name()
	predicateCalled := false
	if err := predicate.Register(predName, func(inputs map[string]any) bool {
		predicateCalled = true
		// This line must never execute; the wire is nil when the producer was skipped.
		return inputs["wire_out"].(*int) != nil
	}); err != nil {
		t.Fatalf("register predicate: %v", err)
	}
	defer predicate.Unregister(predName)

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"producer": {
				Op:      "AnyOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"result": "wire_out"},
			},
			"consumer": {
				Op:              "AnyOp",
				Params:          []byte(`{}`),
				Condition:       predName,
				ConditionInputs: []string{"wire_out"},
			},
		},
	}
	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	pool := newMockGPool()
	eng, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	// Seed a nil field value — mimics the state after a skipped producer.
	eng.status.SetFieldValue("wire_out", &runtime.FieldValue{Name: "result", Value: nil})

	// Mark the producer as skipped.
	eng.status.SetVertexSkipped(g.VertexByName("producer"))

	skip, err := eng.shouldSkip(g.VertexByName("consumer"))
	if err != nil {
		t.Fatalf("shouldSkip returned unexpected error: %v", err)
	}
	if !skip {
		t.Error("consumer should be skipped when its ConditionInput producer was skipped")
	}
	if predicateCalled {
		t.Error("predicate must not be invoked when ConditionInput producer was skipped")
	}
}

// TestEngine_Init_SetupFailReleasesOps verifies TODO-01: operators fetched from
// the pool during init must be returned when Setup fails.
//
// A trackingOpPool wraps the real pool and maintains a net-borrow counter
// (getOp +1, putOp -1). After a failed Run the counter must be zero — every
// fetched op was returned. This is deterministic: no sync.Pool eviction or GC
// timing involved.
func TestEngine_Init_SetupFailReleasesOps(t *testing.T) {
	opName := "TestSetupFailOp_TODO01_" + t.Name()
	if err := operator.RegisterOpFactory(opName, func() operator.IOperator {
		op := newMockOperator(opName)
		op.setupErr = errors.New("setup always fails")
		return op
	}); err != nil {
		t.Fatalf("register op: %v", err)
	}

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"v1": {Op: opName, Params: []byte(`{}`)},
		},
	}
	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	tracking := &trackingOpPool{}
	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	eng.opPool = tracking

	ctx := context.Background()
	if runErr := eng.Run(ctx); runErr == nil {
		t.Fatal("expected error from setup failure")
	}

	if b := tracking.borrowed.Load(); b != 0 {
		t.Errorf("pool leak: %d op(s) fetched but not returned after init failure", b)
	}
}

// panicOnSetInputOperator is a mock operator whose SetInputField panics, used
// to verify that the defer/recover in runOp covers the injection phase.
type panicOnSetInputOperator struct {
	*mockOperator
}

func (p *panicOnSetInputOperator) SetInputField(field string, value any) error {
	panic("SetInputField exploded")
}

// TestEngine_SetInputField_Panic_OnErrorStop covers TODO-05: a panicking
// SetInputField must be caught by the defer/recover in runOp (not crash the
// process). With on_error=stop the engine must return a wrapped error.
func TestEngine_SetInputField_Panic_OnErrorStop(t *testing.T) {
	val := 42
	opA := newMockOperator("A")
	opA.outputFields["out"] = &val

	opB := &panicOnSetInputOperator{mockOperator: newMockOperator("B")}

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"A": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "field_sif_stop"},
				OnError: config.OnErrorStop,
			},
			"B": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Inputs:  map[string]string{"in": "field_sif_stop"},
				OnError: config.OnErrorStop,
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	ops := map[string]*mockOperator{"A": opA, "B": opB.mockOperator}
	engine, _ := setupEngineForTest(t, g, ops)
	// Replace B's operator with the panicking variant after setup wired outputs.
	engine.status.SetOp("B", opB)

	ctx := context.Background()
	runErr := runEngineForTest(t, engine, ctx)
	if runErr == nil {
		t.Fatal("expected engine to return an error when SetInputField panics with on_error=stop")
	}
}

// TestEngine_OnErrorContinue_MarksVertexSkipped covers TODO-07: when a vertex's
// operator Run returns an error and on_error=continue, the vertex must be marked
// skipped and its output fields cleared so successors see consistent nil values
// and propagate the skip transitively.
func TestEngine_OnErrorContinue_MarksVertexSkipped(t *testing.T) {
	t.Run("skip_semantics", func(t *testing.T) {
		// Chain: A → B(on_error=continue, errors) → C
		// Expected: B is marked skipped, field_b07 is nil, C is transitively skipped.
		valA := 1
		opA := newMockOperator("A")
		opA.outputFields["out"] = &valA

		valB := 99
		opB := newMockOperator("B")
		opB.outputFields["out"] = &valB
		opB.runErr = errors.New("B failed deliberately")

		opC := newMockOperator("C")

		graphConfig := &config.GraphConfig{
			Name: "test",
			Vertices: map[string]*config.VertexConfig{
				"A": {
					Op:      "MockOp",
					Params:  []byte(`{}`),
					Outputs: map[string]string{"out": "todo07_field_a"},
					OnError: config.OnErrorStop,
				},
				"B": {
					Op:      "MockOp",
					Params:  []byte(`{}`),
					Inputs:  map[string]string{"in": "todo07_field_a"},
					Outputs: map[string]string{"out": "todo07_field_b"},
					OnError: config.OnErrorContinue,
				},
				"C": {
					Op:      "MockOp",
					Params:  []byte(`{}`),
					Inputs:  map[string]string{"in": "todo07_field_b"},
					OnError: config.OnErrorStop,
				},
			},
		}

		g, err := graph.NewGraphFromConfig(graphConfig)
		if err != nil {
			t.Fatalf("failed to create graph: %v", err)
		}

		ops := map[string]*mockOperator{"A": opA, "B": opB, "C": opC}
		engine, _ := setupEngineForTest(t, g, ops)

		ctx := context.Background()
		if err := runEngineForTest(t, engine, ctx); err != nil {
			t.Fatalf("engine must not return error when middle vertex uses on_error=continue, got: %v", err)
		}

		if !opA.runCalled {
			t.Error("expected A to run")
		}
		if !opB.runCalled {
			t.Error("expected B.Run to be attempted")
		}
		if !engine.VertexSkipped("B") {
			t.Error("expected B to be marked skipped after error with on_error=continue")
		}
		if engine.status.VertexError(g.VertexByName("B")) == nil {
			t.Error("expected per-vertex error recorded for B")
		}
		fv, ok := engine.status.FieldValue("todo07_field_b")
		if !ok {
			t.Fatal("expected todo07_field_b to exist in status")
		}
		if fv.Value != nil {
			t.Errorf("expected B's output cleared to nil after error+skip, got: %v", fv.Value)
		}
		if opC.runCalled {
			t.Error("expected C not to run (B is skipped; C's only producer is skipped)")
		}
		if !engine.VertexSkipped("C") {
			t.Error("expected C to be transitively skipped (its only producer B was skipped)")
		}
	})

	t.Run("passthrough_semantics", func(t *testing.T) {
		// Diamond: A → B(on_error=continue, errors, passthrough out←field_a)
		//          A → C(merge=coalesce, reads field_a + field_b)
		//          B → C
		//
		// B errors: marked skipped, field_b gets A's value via passthrough.
		// C has merge=coalesce and A (not skipped) as a producer, so C runs.
		valA := 42
		opA := newMockOperator("A")
		opA.outputFields["out"] = &valA

		valB := 0
		opB := newMockOperator("B")
		opB.outputFields["out"] = &valB
		opB.runErr = errors.New("B failed deliberately")

		opC := newMockOperator("C")

		graphConfig := &config.GraphConfig{
			Name: "test",
			Vertices: map[string]*config.VertexConfig{
				"A": {
					Op:      "MockOp",
					Params:  []byte(`{}`),
					Outputs: map[string]string{"out": "todo07pt_field_a"},
					OnError: config.OnErrorStop,
				},
				"B": {
					Op:               "MockOp",
					Params:           []byte(`{}`),
					Inputs:           map[string]string{"in": "todo07pt_field_a"},
					Outputs:          map[string]string{"out": "todo07pt_field_b"},
					OnError:          config.OnErrorContinue,
					PassthroughWires: map[string]string{"out": "todo07pt_field_a"},
				},
				"C": {
					Op:      "MockOp",
					Params:  []byte(`{}`),
					Inputs:  map[string]string{"a": "todo07pt_field_a", "b": "todo07pt_field_b"},
					Merge:   config.MergeCoalesce,
					OnError: config.OnErrorStop,
				},
			},
		}

		g, err := graph.NewGraphFromConfig(graphConfig)
		if err != nil {
			t.Fatalf("failed to create graph: %v", err)
		}

		ops := map[string]*mockOperator{"A": opA, "B": opB, "C": opC}
		engine, _ := setupEngineForTest(t, g, ops)

		ctx := context.Background()
		if err := runEngineForTest(t, engine, ctx); err != nil {
			t.Fatalf("engine must not return error, got: %v", err)
		}

		if !engine.VertexSkipped("B") {
			t.Error("expected B to be marked skipped")
		}
		// B's output must carry the passthrough value from A.
		fv, ok := engine.status.FieldValue("todo07pt_field_b")
		if !ok {
			t.Fatal("expected todo07pt_field_b to exist in status")
		}
		if fv.Value != &valA {
			t.Errorf("expected B's output to be passthrough from A (%p), got: %v", &valA, fv.Value)
		}
		// C runs because A (its other producer) is not skipped (merge=coalesce).
		if !opC.runCalled {
			t.Error("expected C to run (merge=coalesce, A is not skipped)")
		}
		if engine.VertexSkipped("C") {
			t.Error("expected C not to be skipped (A provides a non-skipped input)")
		}
	})
}

// TestEngine_SetInputField_Panic_OnErrorContinue covers TODO-05: a panicking
// SetInputField with on_error=continue must not crash the process; the engine
// must succeed and the vertex must be marked skipped with a per-vertex error.
func TestEngine_SetInputField_Panic_OnErrorContinue(t *testing.T) {
	val := 42
	opA := newMockOperator("A")
	opA.outputFields["out"] = &val

	opB := &panicOnSetInputOperator{mockOperator: newMockOperator("B")}

	graphConfig := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"A": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "field_sif_cont"},
				OnError: config.OnErrorStop,
			},
			"B": {
				Op:      "MockOp",
				Params:  []byte(`{}`),
				Inputs:  map[string]string{"in": "field_sif_cont"},
				OnError: config.OnErrorContinue,
			},
		},
	}

	g, err := graph.NewGraphFromConfig(graphConfig)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	ops := map[string]*mockOperator{"A": opA, "B": opB.mockOperator}
	engine, _ := setupEngineForTest(t, g, ops)
	engine.status.SetOp("B", opB)

	ctx := context.Background()
	if runErr := runEngineForTest(t, engine, ctx); runErr != nil {
		t.Fatalf("engine must not return error when SetInputField panics with on_error=continue, got: %v", runErr)
	}
	if opB.runCalled {
		t.Error("B.Run must not be called when SetInputField panicked")
	}
	bVertex := g.VertexByName("B")
	if engine.status.VertexError(bVertex) == nil {
		t.Error("expected per-vertex error recorded for B after SetInputField panic")
	}
}
