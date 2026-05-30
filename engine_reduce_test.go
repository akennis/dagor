package dagor

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/operator"
	"github.com/akennis/dagor/predicate"
	"github.com/akennis/dagor/reducer"
)

// withReducer registers a reducer in the global registry and unregisters it
// when the test ends, preventing cross-test leakage.
func withReducer(t *testing.T, name string, fn reducer.Reducer) {
	t.Helper()
	reducer.MustReplace(name, fn)
	t.Cleanup(func() { reducer.Unregister(name) })
}

// sumReducer adds two int values. Both acc and item are plain int (not *int)
// because reduce operates on the concrete slice element values.
func sumReducer(acc any, item any) any {
	return acc.(int) + item.(int)
}

// registerReduceSrcOp registers a source operator that outputs outputVal under
// field "out" using opName as the operator type name.
func registerReduceSrcOp(t *testing.T, opName string, outputVal any) {
	t.Helper()
	var rc atomic.Int32
	_ = operator.RegisterOpFactory(opName, func() operator.IOperator {
		return newTrackingOp(nil, outputVal, &rc)
	})
}

// TestReduceVertex_SumIntegers is the primary integration test.
// A filter produces a []any of positive ints; reduce sums them.
func TestReduceVertex_SumIntegers(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	srcOpName := "TestReduceSrc_sum"
	registerReduceSrcOp(t, srcOpName, &items)
	withReducer(t, "reduce_sum", sumReducer)

	g, err := graph.NewBuilder("reduce_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("reduce").
		Input("In", "items_wire").
		ReduceBy("reduce_sum").
		CollectInto("total_wire").
		Build()
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, ok := eng.GetOutput("total_wire")
	if !ok {
		t.Fatal("total_wire not found in engine output")
	}
	if out != 15 {
		t.Errorf("expected 15, got %v (%T)", out, out)
	}
}

// TestReduceVertex_WithInitWire verifies that InitFrom supplies the initial
// accumulator from a separate producer vertex.
// The init wire delivers *int (dagor's pointer-based wire convention), so the
// reducer must dereference it on the first call before switching to plain int.
func TestReduceVertex_WithInitWire(t *testing.T) {
	items := []int{1, 2, 3}
	srcOpName := "TestReduceSrc_init_wire"
	registerReduceSrcOp(t, srcOpName, &items)

	initVal := 100
	initOpName := "TestReduceInit_init_wire"
	var rc atomic.Int32
	_ = operator.RegisterOpFactory(initOpName, func() operator.IOperator {
		return newTrackingOp(nil, &initVal, &rc)
	})

	// The init wire carries *int (pointer convention); subsequent acc values are int.
	withReducer(t, "reduce_sum_init", func(acc any, item any) any {
		var a int
		switch v := acc.(type) {
		case *int:
			a = *v
		case int:
			a = v
		}
		return a + item.(int)
	})

	g, err := graph.NewBuilder("reduce_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("init_src").
		Op(initOpName).
		Output("out", "init_wire").
		Vertex("reduce").
		Input("In", "items_wire").
		ReduceBy("reduce_sum_init").
		InitFrom("init_wire").
		CollectInto("total_wire").
		Build()
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, ok := eng.GetOutput("total_wire")
	if !ok {
		t.Fatal("total_wire not found")
	}
	// 100 + 1 + 2 + 3 = 106
	if out != 106 {
		t.Errorf("expected 106, got %v (%T)", out, out)
	}
}

// TestReduceVertex_EmptySlice_NoInit verifies that an empty slice with no
// InitWire produces a nil result without error.
func TestReduceVertex_EmptySlice_NoInit(t *testing.T) {
	items := []int{}
	srcOpName := "TestReduceSrc_empty_no_init"
	registerReduceSrcOp(t, srcOpName, &items)
	withReducer(t, "reduce_empty_no_init", sumReducer)

	g, err := graph.NewBuilder("reduce_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("reduce").
		Input("In", "items_wire").
		ReduceBy("reduce_empty_no_init").
		CollectInto("total_wire").
		Build()
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, ok := eng.GetOutput("total_wire")
	if !ok {
		t.Fatal("total_wire not found")
	}
	if out != nil {
		t.Errorf("expected nil for empty slice with no init, got %v", out)
	}
}

// TestReduceVertex_SingleElement verifies that a single-element slice with no
// InitWire returns that element unchanged.
func TestReduceVertex_SingleElement(t *testing.T) {
	items := []int{42}
	srcOpName := "TestReduceSrc_single"
	registerReduceSrcOp(t, srcOpName, &items)
	withReducer(t, "reduce_single", sumReducer)

	g, err := graph.NewBuilder("reduce_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("reduce").
		Input("In", "items_wire").
		ReduceBy("reduce_single").
		CollectInto("total_wire").
		Build()
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, ok := eng.GetOutput("total_wire")
	if !ok {
		t.Fatal("total_wire not found")
	}
	if out != 42 {
		t.Errorf("expected 42, got %v", out)
	}
}

// TestReduceVertex_UnregisteredReducer verifies that an unregistered reducer
// name causes the engine to return an error.
func TestReduceVertex_UnregisteredReducer(t *testing.T) {
	items := []int{1, 2, 3}
	srcOpName := "TestReduceSrc_unreg"
	registerReduceSrcOp(t, srcOpName, &items)

	cfg := &config.GraphConfig{
		Name: "reduce_graph",
		Vertices: map[string]*config.VertexConfig{
			"src": {
				Op:      srcOpName,
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "items_wire"},
			},
			"reduce": {
				Inputs: map[string]string{"In": "items_wire"},
				Reduce: &config.ReduceConfig{
					Reducer:     "reducer_not_registered",
					ResultsWire: "total_wire",
				},
			},
		},
	}

	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if err := eng.Run(context.Background()); err == nil {
		t.Fatal("expected error for unregistered reducer, got nil")
	}
}

// TestReduceVertex_SkipsWhenUpstreamSkipped verifies that the reduce vertex is
// skipped when its input producer was skipped, leaving ResultsWire nil.
func TestReduceVertex_SkipsWhenUpstreamSkipped(t *testing.T) {
	items := []int{1, 2, 3}
	srcOpName := "TestReduceSrc_skip_upstream"
	registerReduceSrcOp(t, srcOpName, &items)
	withReducer(t, "reduce_skip_upstream", sumReducer)
	predicate.MustReplace("always_false_reduce", func(map[string]any) bool { return false })
	t.Cleanup(func() { predicate.Unregister("always_false_reduce") })

	g, err := graph.NewBuilder("reduce_graph").
		Vertex("src").
		Op(srcOpName).
		Condition("always_false_reduce").
		Output("out", "items_wire").
		Vertex("reduce").
		Input("In", "items_wire").
		ReduceBy("reduce_skip_upstream").
		CollectInto("total_wire").
		Build()
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !eng.VertexSkipped("src") {
		t.Error("expected src vertex to be skipped")
	}
	if !eng.VertexSkipped("reduce") {
		t.Error("expected reduce vertex to be skipped due to skipped upstream")
	}

	out, ok := eng.GetOutput("total_wire")
	if !ok {
		t.Fatal("total_wire not found")
	}
	if out != nil {
		t.Errorf("expected nil for skipped reduce vertex, got %v", out)
	}
}

// TestReduceVertex_SkipsWhenInitWireSkipped verifies that the reduce vertex is
// skipped when the InitWire producer was skipped.
func TestReduceVertex_SkipsWhenInitWireSkipped(t *testing.T) {
	items := []int{1, 2, 3}
	srcOpName := "TestReduceSrc_skip_init"
	registerReduceSrcOp(t, srcOpName, &items)

	initVal := 0
	initOpName := "TestReduceInit_skip_init"
	var rc atomic.Int32
	_ = operator.RegisterOpFactory(initOpName, func() operator.IOperator {
		return newTrackingOp(nil, &initVal, &rc)
	})

	withReducer(t, "reduce_skip_init", sumReducer)
	predicate.MustReplace("always_false_reduce_init", func(map[string]any) bool { return false })
	t.Cleanup(func() { predicate.Unregister("always_false_reduce_init") })

	g, err := graph.NewBuilder("reduce_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("init_src").
		Op(initOpName).
		Condition("always_false_reduce_init").
		Output("out", "init_wire").
		Vertex("reduce").
		Input("In", "items_wire").
		ReduceBy("reduce_skip_init").
		InitFrom("init_wire").
		CollectInto("total_wire").
		Build()
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !eng.VertexSkipped("init_src") {
		t.Error("expected init_src to be skipped")
	}
	if !eng.VertexSkipped("reduce") {
		t.Error("expected reduce to be skipped when InitWire producer is skipped")
	}

	out, ok := eng.GetOutput("total_wire")
	if !ok {
		t.Fatal("total_wire not found")
	}
	if out != nil {
		t.Errorf("expected nil for skipped reduce vertex, got %v", out)
	}
}

// TestReduceVertex_DownstreamReceivesResult verifies end-to-end that an
// operator downstream of the reduce vertex correctly receives the accumulated value.
func TestReduceVertex_DownstreamReceivesResult(t *testing.T) {
	items := []int{10, 20, 30}
	srcOpName := "TestReduceSrc_downstream"
	registerReduceSrcOp(t, srcOpName, &items)
	withReducer(t, "reduce_downstream", sumReducer)

	var receivedValue any
	consumerOpName := "TestReduceConsumer_downstream"
	_ = operator.RegisterOpFactory(consumerOpName, func() operator.IOperator {
		return &reduceConsumerOp{received: &receivedValue}
	})

	g, err := graph.NewBuilder("reduce_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("reduce").
		Input("In", "items_wire").
		ReduceBy("reduce_downstream").
		CollectInto("total_wire").
		Vertex("consumer").
		Op(consumerOpName).
		Input("Result", "total_wire").
		Build()
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if receivedValue != 60 {
		t.Errorf("expected consumer to receive 60, got %v (%T)", receivedValue, receivedValue)
	}
}

// TestReduceVertex_MapThenReduce verifies the canonical map/reduce pipeline:
// a map vertex fans out a sub-graph over a slice, then reduce aggregates results.
func TestReduceVertex_MapThenReduce(t *testing.T) {
	items := []int{1, 2, 3, 4}
	srcOpName := "TestReduceSrc_map_then_reduce"
	registerReduceSrcOp(t, srcOpName, &items)

	// doubleOpName squares each element inside the map sub-graph.
	doubleOpName := "TestReduceDouble_map_then_reduce"
	_ = operator.RegisterOpFactory(doubleOpName, func() operator.IOperator {
		return &doubleIntOp{}
	})

	withReducer(t, "reduce_map_then_reduce", sumReducer)

	g, err := graph.NewBuilder("map_reduce_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("mapv").
		Input("In", "items_wire").
		MapOver("item").
		SubVertex("double").
		Op(doubleOpName).
		Input("in", "item").
		Output("out", "doubled").
		CollectInto("doubled", "doubled_wire").
		Vertex("reduce").
		Input("In", "doubled_wire").
		ReduceBy("reduce_map_then_reduce").
		CollectInto("total_wire").
		Build()
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, ok := eng.GetOutput("total_wire")
	if !ok {
		t.Fatal("total_wire not found")
	}
	// doubled_wire is *[]any; map results are the concrete values from the sub-graph.
	// Each element (1,2,3,4) is doubled to (2,4,6,8); sum = 20.
	if out != 20 {
		t.Errorf("expected 20, got %v (%T)", out, out)
	}
}

// reduceConsumerOp records the value it receives on its "Result" input field.
type reduceConsumerOp struct {
	received *any
	inputs   map[string]any
	outputs  map[string]any
}

func (o *reduceConsumerOp) Setup(_ *config.Params) error {
	o.inputs = make(map[string]any)
	o.outputs = make(map[string]any)
	return nil
}
func (o *reduceConsumerOp) Run(_ context.Context) error  { return nil }
func (o *reduceConsumerOp) Reset() error                 { return nil }
func (o *reduceConsumerOp) InputFields() map[string]any  { return o.inputs }
func (o *reduceConsumerOp) OutputFields() map[string]any { return o.outputs }
func (o *reduceConsumerOp) ResetFields()                 {}
func (o *reduceConsumerOp) SetInputField(f string, v any) error {
	if f == "Result" {
		*o.received = v
	}
	return nil
}

// doubleIntOp receives an int via "in" and outputs int*2 via "out".
type doubleIntOp struct {
	inVal  *int
	outVal int
}

func (o *doubleIntOp) Setup(_ *config.Params) error { return nil }
func (o *doubleIntOp) Run(_ context.Context) error {
	if o.inVal != nil {
		o.outVal = *o.inVal * 2
	}
	return nil
}
func (o *doubleIntOp) Reset() error { o.inVal = nil; o.outVal = 0; return nil }
func (o *doubleIntOp) InputFields() map[string]any {
	return map[string]any{"in": &o.inVal}
}
func (o *doubleIntOp) OutputFields() map[string]any {
	return map[string]any{"out": &o.outVal}
}
func (o *doubleIntOp) ResetFields() { o.inVal = nil; o.outVal = 0 }
func (o *doubleIntOp) SetInputField(f string, v any) error {
	if f == "in" {
		ptr, ok := v.(*int)
		if !ok {
			return nil
		}
		o.inVal = ptr
	}
	return nil
}
