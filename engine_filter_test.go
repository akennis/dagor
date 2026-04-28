package dagor

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/graph"
	"github.com/wwz16/dagor/operator"
	"github.com/wwz16/dagor/predicate"
)

// withFilterPredicate registers predicateName in the global registry and
// unregisters it when the test ends. Prevents cross-test leakage.
func withFilterPredicate(t *testing.T, name string, fn predicate.Predicate) {
	t.Helper()
	predicate.MustReplace(name, fn)
	t.Cleanup(func() { predicate.Unregister(name) })
}

// positiveIntPredicate returns true when inputs["item"] is a *int > 0.
func positiveIntPredicate(inputs map[string]any) bool {
	ptr, ok := inputs["item"].(*int)
	return ok && ptr != nil && *ptr > 0
}

// registerFilterSrcOp registers a source operator that outputs outputVal under
// field "out", using opName as the operator type name. Safe to call once per
// unique opName across tests; duplicate registration is a no-op (returns error
// which we ignore here since factory tests register different names).
func registerFilterSrcOp(t *testing.T, opName string, outputVal any) {
	t.Helper()
	var rc atomic.Int32
	_ = operator.RegisterOpFactory(opName, func() operator.IOperator {
		return newTrackingOp(nil, outputVal, &rc)
	})
}

// TestFilterVertex_KeepsPositiveIntegers is the primary integration test.
// Verifies that a filter vertex applies the predicate element-wise and retains
// only matching elements in the correct order.
func TestFilterVertex_KeepsPositiveIntegers(t *testing.T) {
	items := []int{1, -2, 3, -4}
	srcOpName := "TestFilterSrc_keep_positives"
	registerFilterSrcOp(t, srcOpName, &items)
	withFilterPredicate(t, "filter_positive", positiveIntPredicate)

	g, err := graph.NewBuilder("filter_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("filter").
		Input("In", "items_wire").
		FilterBy("filter_positive").
		CollectInto("filtered_wire").
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

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		t.Fatal("filtered_wire not found in engine output")
	}
	ptr, ok := out.(*[]any)
	if !ok {
		t.Fatalf("expected *[]any, got %T", out)
	}
	got := *ptr
	want := []any{1, 3}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("result[%d]: expected %v, got %v", i, w, got[i])
		}
	}
}

func TestFilterVertex_AllPass(t *testing.T) {
	items := []int{1, 2, 3}
	srcOpName := "TestFilterSrc_all_pass"
	registerFilterSrcOp(t, srcOpName, &items)
	withFilterPredicate(t, "filter_all_pass", positiveIntPredicate)

	g, err := graph.NewBuilder("filter_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("filter").
		Input("In", "items_wire").
		FilterBy("filter_all_pass").
		CollectInto("filtered_wire").
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

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		t.Fatal("filtered_wire not found")
	}
	got := *(out.(*[]any))
	if len(got) != 3 {
		t.Fatalf("expected 3 elements, got %d: %v", len(got), got)
	}
}

func TestFilterVertex_NonePass(t *testing.T) {
	items := []int{-1, -2, -3}
	srcOpName := "TestFilterSrc_none_pass"
	registerFilterSrcOp(t, srcOpName, &items)
	withFilterPredicate(t, "filter_none_pass", positiveIntPredicate)

	g, err := graph.NewBuilder("filter_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("filter").
		Input("In", "items_wire").
		FilterBy("filter_none_pass").
		CollectInto("filtered_wire").
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

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		t.Fatal("filtered_wire not found")
	}
	got := *(out.(*[]any))
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestFilterVertex_EmptySlice(t *testing.T) {
	items := []int{}
	srcOpName := "TestFilterSrc_empty"
	registerFilterSrcOp(t, srcOpName, &items)
	withFilterPredicate(t, "filter_empty", positiveIntPredicate)

	g, err := graph.NewBuilder("filter_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("filter").
		Input("In", "items_wire").
		FilterBy("filter_empty").
		CollectInto("filtered_wire").
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

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		t.Fatal("filtered_wire not found")
	}
	got := *(out.(*[]any))
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

// TestFilterVertex_CustomItemKey verifies that the ItemKey override is respected
// when the predicate reads from a non-default key.
func TestFilterVertex_CustomItemKey(t *testing.T) {
	items := []int{5, -1, 7}
	srcOpName := "TestFilterSrc_custom_key"
	registerFilterSrcOp(t, srcOpName, &items)
	withFilterPredicate(t, "filter_custom_key", func(inputs map[string]any) bool {
		ptr, ok := inputs["element"].(*int) // custom key
		return ok && ptr != nil && *ptr > 0
	})

	g, err := graph.NewBuilder("filter_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("filter").
		Input("In", "items_wire").
		FilterBy("filter_custom_key").
		ItemKey("element").
		CollectInto("filtered_wire").
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

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		t.Fatal("filtered_wire not found")
	}
	got := *(out.(*[]any))
	want := []any{5, 7}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

// TestFilterVertex_UnregisteredPredicate verifies that an unregistered predicate
// name causes the engine to return an error (default OnErrorStop).
func TestFilterVertex_UnregisteredPredicate(t *testing.T) {
	items := []int{1, 2, 3}
	srcOpName := "TestFilterSrc_unreg"
	registerFilterSrcOp(t, srcOpName, &items)

	cfg := &config.GraphConfig{
		Name: "filter_graph",
		Vertices: map[string]*config.VertexConfig{
			"src": {
				Op:      srcOpName,
				Params:  []byte(`{}`),
				Outputs: map[string]string{"out": "items_wire"},
			},
			"filter": {
				Inputs: map[string]string{"In": "items_wire"},
				Filter: &config.FilterConfig{
					Predicate:   "predicate_not_registered",
					ResultsWire: "filtered_wire",
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
		t.Fatal("expected error for unregistered predicate, got nil")
	}
}

// TestFilterVertex_SkipsWhenUpstreamSkipped verifies that a filter vertex is
// skipped (output remains nil) when the source vertex is skipped.
func TestFilterVertex_SkipsWhenUpstreamSkipped(t *testing.T) {
	items := []int{1, 2, 3}
	srcOpName := "TestFilterSrc_skip_upstream"
	registerFilterSrcOp(t, srcOpName, &items)
	withFilterPredicate(t, "filter_skip_upstream", positiveIntPredicate)
	// always_false causes the src vertex to be skipped.
	withFilterPredicate(t, "always_false_filter", func(map[string]any) bool { return false })

	g, err := graph.NewBuilder("filter_graph").
		Vertex("src").
		Op(srcOpName).
		Condition("always_false_filter").
		Output("out", "items_wire").
		Vertex("filter").
		Input("In", "items_wire").
		FilterBy("filter_skip_upstream").
		CollectInto("filtered_wire").
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

	// src was skipped, so filter should also be skipped and its output nil.
	if !eng.VertexSkipped("src") {
		t.Error("expected src vertex to be skipped")
	}
	if !eng.VertexSkipped("filter") {
		t.Error("expected filter vertex to be skipped due to skipped upstream")
	}

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		t.Fatal("filtered_wire not found in engine output")
	}
	if out != nil {
		t.Errorf("expected nil output for skipped filter vertex, got %v", out)
	}
}

// TestFilterVertex_DownstreamReceivesFilteredSlice verifies end-to-end that an
// operator downstream of the filter correctly receives the *[]any wire value.
func TestFilterVertex_DownstreamReceivesFilteredSlice(t *testing.T) {
	items := []int{10, -5, 20, -15, 30}
	srcOpName := "TestFilterSrc_downstream"
	registerFilterSrcOp(t, srcOpName, &items)
	withFilterPredicate(t, "filter_downstream", positiveIntPredicate)

	// consumerOp records what it receives in its input field.
	var receivedValue any
	consumerOpName := "TestFilterConsumer_downstream"
	_ = operator.RegisterOpFactory(consumerOpName, func() operator.IOperator {
		return &sliceConsumerOp{received: &receivedValue}
	})

	g, err := graph.NewBuilder("filter_graph").
		Vertex("src").
		Op(srcOpName).
		Output("out", "items_wire").
		Vertex("filter").
		Input("In", "items_wire").
		FilterBy("filter_downstream").
		CollectInto("filtered_wire").
		Vertex("consumer").
		Op(consumerOpName).
		Input("Slice", "filtered_wire").
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

	ptr, ok := receivedValue.(*[]any)
	if !ok {
		t.Fatalf("expected consumer to receive *[]any, got %T", receivedValue)
	}
	got := *ptr
	want := []any{10, 20, 30}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("filtered[%d]: expected %v, got %v", i, w, got[i])
		}
	}
}

// sliceConsumerOp is a minimal operator that records the *[]any it receives.
type sliceConsumerOp struct {
	received *any
	inputs   map[string]any
	outputs  map[string]any
}

func (o *sliceConsumerOp) Setup(_ *config.Params) error {
	o.inputs = make(map[string]any)
	o.outputs = make(map[string]any)
	return nil
}
func (o *sliceConsumerOp) Run(_ context.Context) error  { return nil }
func (o *sliceConsumerOp) Reset() error                 { return nil }
func (o *sliceConsumerOp) InputFields() map[string]any  { return o.inputs }
func (o *sliceConsumerOp) OutputFields() map[string]any { return o.outputs }
func (o *sliceConsumerOp) ResetFields()                 {}
func (o *sliceConsumerOp) SetInputField(f string, v any) error {
	if f == "Slice" {
		*o.received = v
	}
	return nil
}
