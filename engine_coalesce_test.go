package dagor

// Integration tests for the coalesce merge strategy.
//
// These tests exercise the full engine path — graph construction, init(),
// conditional skip propagation, and CoalesceOp field injection — so that the
// coalesce semantics are tested end-to-end rather than in isolation.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	_ "github.com/akennis/dagor/operator/builtin" // registers CoalesceStringOp, CoalesceIntOp …

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/operator"
	"github.com/akennis/dagor/predicate"
)

// ─── Test-local operator types ────────────────────────────────────────────────
//
// These are minimal implementations of IOperator used only by the coalesce
// engine integration tests.  They are registered once per test binary.

// coalTestSourceOp emits a configurable string value.
type coalTestSourceOp struct {
	val    string
	Result string
}

func (o *coalTestSourceOp) Setup(p *config.Params) error        { o.val = p.GetString("val", ""); return nil }
func (o *coalTestSourceOp) Run(_ context.Context) error         { o.Result = o.val; return nil }
func (o *coalTestSourceOp) Reset() error                        { return nil }
func (o *coalTestSourceOp) InputFields() map[string]any         { return map[string]any{} }
func (o *coalTestSourceOp) OutputFields() map[string]any        { return map[string]any{"Result": &o.Result} }
func (o *coalTestSourceOp) SetInputField(_ string, _ any) error { return nil }
func (o *coalTestSourceOp) ResetFields()                        { o.Result = "" }

// coalTestDetOp is the "deterministic / fast-path" branch operator.
// It prefixes its string input with "det:".
type coalTestDetOp struct {
	Input  *string
	Result string
}

func (o *coalTestDetOp) Setup(_ *config.Params) error { return nil }
func (o *coalTestDetOp) Reset() error                 { return nil }
func (o *coalTestDetOp) Run(_ context.Context) error {
	if o.Input != nil {
		o.Result = "det:" + *o.Input
	}
	return nil
}
func (o *coalTestDetOp) InputFields() map[string]any  { return map[string]any{"Input": &o.Input} }
func (o *coalTestDetOp) OutputFields() map[string]any { return map[string]any{"Result": &o.Result} }
func (o *coalTestDetOp) SetInputField(field string, value any) error {
	if value == nil {
		return nil
	}
	if field != "Input" {
		return fmt.Errorf("unknown field %q", field)
	}
	val, ok := value.(*string)
	if !ok {
		return fmt.Errorf("expected *string, got %T", value)
	}
	o.Input = val
	return nil
}
func (o *coalTestDetOp) ResetFields() { o.Input = nil; o.Result = "" }

// coalTestAiOp is the "AI / slow-path" branch operator.
// It prefixes its string input with "ai:".
type coalTestAiOp struct {
	Input  *string
	Result string
}

func (o *coalTestAiOp) Setup(_ *config.Params) error { return nil }
func (o *coalTestAiOp) Reset() error                 { return nil }
func (o *coalTestAiOp) Run(_ context.Context) error {
	if o.Input != nil {
		o.Result = "ai:" + *o.Input
	}
	return nil
}
func (o *coalTestAiOp) InputFields() map[string]any  { return map[string]any{"Input": &o.Input} }
func (o *coalTestAiOp) OutputFields() map[string]any { return map[string]any{"Result": &o.Result} }
func (o *coalTestAiOp) SetInputField(field string, value any) error {
	if value == nil {
		return nil
	}
	if field != "Input" {
		return fmt.Errorf("unknown field %q", field)
	}
	val, ok := value.(*string)
	if !ok {
		return fmt.Errorf("expected *string, got %T", value)
	}
	o.Input = val
	return nil
}
func (o *coalTestAiOp) ResetFields() { o.Input = nil; o.Result = "" }

// ─── Registration ─────────────────────────────────────────────────────────────

var registerCoalesceTestOpsOnce sync.Once

func initCoalesceTestOps() {
	registerCoalesceTestOpsOnce.Do(func() {
		for name, factory := range map[string]func() operator.IOperator{
			"_CoalTestSourceOp": func() operator.IOperator { return &coalTestSourceOp{} },
			"_CoalTestDetOp":    func() operator.IOperator { return &coalTestDetOp{} },
			"_CoalTestAiOp":     func() operator.IOperator { return &coalTestAiOp{} },
		} {
			if err := operator.RegisterOpFactory(name, factory); err != nil {
				panic(fmt.Sprintf("register %s: %v", name, err))
			}
		}
	})
}

// ─── Graph builder ────────────────────────────────────────────────────────────

// buildCoalesceGraph builds the canonical two-branch coalesce graph:
//
//	source ──► det (condition=detPred) ──► merge (merge=coalesce)
//	       └─► ai  (condition=aiPred)  ──►
//
// detPred / aiPred are the names of already-registered predicates.
func buildCoalesceGraph(t *testing.T, sourceVal, detPred, aiPred string) *graph.Graph {
	t.Helper()
	srcParams, _ := json.Marshal(map[string]string{"val": sourceVal})
	cfg := &config.GraphConfig{
		Name: "coalesce_test",
		Vertices: map[string]*config.VertexConfig{
			"source": {
				Op:      "_CoalTestSourceOp",
				Params:  srcParams,
				Outputs: map[string]string{"Result": "src_out"},
			},
			"det": {
				Op:        "_CoalTestDetOp",
				Condition: detPred,
				Inputs:    map[string]string{"Input": "src_out"},
				Outputs:   map[string]string{"Result": "det_result"},
			},
			"ai": {
				Op:        "_CoalTestAiOp",
				Condition: aiPred,
				Inputs:    map[string]string{"Input": "src_out"},
				Outputs:   map[string]string{"Result": "ai_result"},
			},
			"merge": {
				Op:      "CoalesceStringOp",
				Merge:   config.MergeCoalesce,
				Inputs:  map[string]string{"A": "det_result", "B": "ai_result"},
				Outputs: map[string]string{"Result": "final_result"},
			},
		},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewGraphFromConfig: %v", err)
	}
	return g
}

// runCoalesceEngine runs the engine and returns the final_result string.
func runCoalesceEngine(t *testing.T, g *graph.Graph) (string, bool) {
	t.Helper()
	pool := newMockGPool()
	eng, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	ctx := context.Background()
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	if eng.VertexSkipped("merge") {
		return "", false
	}
	val, ok := eng.GetOutput("final_result")
	if !ok {
		t.Fatal("GetOutput(final_result) not found")
	}
	return *val.(*string), true
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestCoalesceMerge_BranchA_Runs: det runs, ai is skipped.
// The coalesce vertex must pick det's result.
func TestCoalesceMerge_BranchA_Runs(t *testing.T) {
	initCoalesceTestOps()

	detPred := "coal_test_det_true"
	aiPred := "coal_test_ai_false_a"
	if err := predicate.Register(detPred, func(_ map[string]any) bool { return true }); err != nil {
		t.Fatalf("register %s: %v", detPred, err)
	}
	if err := predicate.Register(aiPred, func(_ map[string]any) bool { return false }); err != nil {
		t.Fatalf("register %s: %v", aiPred, err)
	}
	t.Cleanup(func() {
		predicate.Unregister(detPred)
		predicate.Unregister(aiPred)
	})

	g := buildCoalesceGraph(t, "tokyo", detPred, aiPred)
	result, ran := runCoalesceEngine(t, g)

	if !ran {
		t.Fatal("expected merge vertex to run, but it was skipped")
	}
	if result != "det:tokyo" {
		t.Errorf("expected %q, got %q", "det:tokyo", result)
	}
}

// TestCoalesceMerge_BranchB_Runs: det is skipped, ai runs.
// The coalesce vertex must pick ai's result.
func TestCoalesceMerge_BranchB_Runs(t *testing.T) {
	initCoalesceTestOps()

	detPred := "coal_test_det_false_b"
	aiPred := "coal_test_ai_true"
	if err := predicate.Register(detPred, func(_ map[string]any) bool { return false }); err != nil {
		t.Fatalf("register %s: %v", detPred, err)
	}
	if err := predicate.Register(aiPred, func(_ map[string]any) bool { return true }); err != nil {
		t.Fatalf("register %s: %v", aiPred, err)
	}
	t.Cleanup(func() {
		predicate.Unregister(detPred)
		predicate.Unregister(aiPred)
	})

	g := buildCoalesceGraph(t, "london", detPred, aiPred)
	result, ran := runCoalesceEngine(t, g)

	if !ran {
		t.Fatal("expected merge vertex to run, but it was skipped")
	}
	if result != "ai:london" {
		t.Errorf("expected %q, got %q", "ai:london", result)
	}
}

// TestCoalesceMerge_BothRun: both conditions true.
// CoalesceOp should pick A (det) since A is checked first.
func TestCoalesceMerge_BothRun(t *testing.T) {
	initCoalesceTestOps()

	detPred := "coal_test_det_both_true"
	aiPred := "coal_test_ai_both_true"
	if err := predicate.Register(detPred, func(_ map[string]any) bool { return true }); err != nil {
		t.Fatalf("register %s: %v", detPred, err)
	}
	if err := predicate.Register(aiPred, func(_ map[string]any) bool { return true }); err != nil {
		t.Fatalf("register %s: %v", aiPred, err)
	}
	t.Cleanup(func() {
		predicate.Unregister(detPred)
		predicate.Unregister(aiPred)
	})

	g := buildCoalesceGraph(t, "paris", detPred, aiPred)
	result, ran := runCoalesceEngine(t, g)

	if !ran {
		t.Fatal("expected merge vertex to run")
	}
	// CoalesceOp.Run picks the first non-nil (A = det).
	if result != "det:paris" {
		t.Errorf("expected A wins %q, got %q", "det:paris", result)
	}
}

// TestCoalesceMerge_BothSkipped: both branches skipped.
// The coalesce vertex must also be skipped (all producers skipped).
func TestCoalesceMerge_BothSkipped(t *testing.T) {
	initCoalesceTestOps()

	detPred := "coal_test_det_false_c"
	aiPred := "coal_test_ai_false_c"
	if err := predicate.Register(detPred, func(_ map[string]any) bool { return false }); err != nil {
		t.Fatalf("register %s: %v", detPred, err)
	}
	if err := predicate.Register(aiPred, func(_ map[string]any) bool { return false }); err != nil {
		t.Fatalf("register %s: %v", aiPred, err)
	}
	t.Cleanup(func() {
		predicate.Unregister(detPred)
		predicate.Unregister(aiPred)
	})

	g := buildCoalesceGraph(t, "berlin", detPred, aiPred)
	pool := newMockGPool()
	eng, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	ctx := context.Background()
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !eng.VertexSkipped("det") {
		t.Error("expected det to be skipped")
	}
	if !eng.VertexSkipped("ai") {
		t.Error("expected ai to be skipped")
	}
	if !eng.VertexSkipped("merge") {
		t.Error("expected merge to be skipped when all producers are skipped")
	}
}

// TestCoalesceMerge_Unconditional: no conditions — both branches always run.
// Coalesce picks A (det).
func TestCoalesceMerge_Unconditional(t *testing.T) {
	initCoalesceTestOps()

	srcParams, _ := json.Marshal(map[string]string{"val": "madrid"})
	cfg := &config.GraphConfig{
		Name: "coalesce_unconditional",
		Vertices: map[string]*config.VertexConfig{
			"source": {
				Op:      "_CoalTestSourceOp",
				Params:  srcParams,
				Outputs: map[string]string{"Result": "src_out2"},
			},
			"det": {
				Op:      "_CoalTestDetOp",
				Inputs:  map[string]string{"Input": "src_out2"},
				Outputs: map[string]string{"Result": "det_result2"},
			},
			"ai": {
				Op:      "_CoalTestAiOp",
				Inputs:  map[string]string{"Input": "src_out2"},
				Outputs: map[string]string{"Result": "ai_result2"},
			},
			"merge": {
				Op:      "CoalesceStringOp",
				Merge:   config.MergeCoalesce,
				Inputs:  map[string]string{"A": "det_result2", "B": "ai_result2"},
				Outputs: map[string]string{"Result": "final_result2"},
			},
		},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewGraphFromConfig: %v", err)
	}

	pool := newMockGPool()
	eng, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	ctx := context.Background()
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	defer eng.Close(ctx)

	if eng.VertexSkipped("merge") {
		t.Fatal("expected merge to run when branches are unconditional")
	}
	val, ok := eng.GetOutput("final_result2")
	if !ok {
		t.Fatal("GetOutput(final_result2) not found")
	}
	result := *val.(*string)
	if result != "det:madrid" {
		t.Errorf("expected A wins %q, got %q", "det:madrid", result)
	}
}

// TestCoalesceMerge_NInput: three-branch CoalesceNStringOp, second branch runs.
func TestCoalesceMerge_NInput(t *testing.T) {
	initCoalesceTestOps()

	detPred := "coal_n_det_false"
	aiPred := "coal_n_ai_true"
	fbPred := "coal_n_fb_false"
	for name, val := range map[string]bool{
		detPred: false,
		aiPred:  true,
		fbPred:  false,
	} {
		v := val
		if err := predicate.Register(name, func(_ map[string]any) bool { return v }); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		predicate.Unregister(detPred)
		predicate.Unregister(aiPred)
		predicate.Unregister(fbPred)
	})

	// Third branch op: coalTestAiOp reused (prefixes with "ai:"), distinguished by pred.
	srcParams, _ := json.Marshal(map[string]string{"val": "sydney"})
	cfg := &config.GraphConfig{
		Name: "coalesce_n_test",
		Vertices: map[string]*config.VertexConfig{
			"source": {
				Op:      "_CoalTestSourceOp",
				Params:  srcParams,
				Outputs: map[string]string{"Result": "src_out3"},
			},
			"det": {
				Op:        "_CoalTestDetOp",
				Condition: detPred,
				Inputs:    map[string]string{"Input": "src_out3"},
				Outputs:   map[string]string{"Result": "det_result3"},
			},
			"ai": {
				Op:        "_CoalTestAiOp",
				Condition: aiPred,
				Inputs:    map[string]string{"Input": "src_out3"},
				Outputs:   map[string]string{"Result": "ai_result3"},
			},
			"fb": {
				Op:        "_CoalTestDetOp", // reuse det op for the third branch
				Condition: fbPred,
				Inputs:    map[string]string{"Input": "src_out3"},
				Outputs:   map[string]string{"Result": "fb_result3"},
			},
			"merge": {
				Op:      "CoalesceNStringOp",
				Merge:   config.MergeCoalesce,
				Params:  json.RawMessage(`{"n":3}`),
				Inputs:  map[string]string{"Input0": "det_result3", "Input1": "ai_result3", "Input2": "fb_result3"},
				Outputs: map[string]string{"Result": "final_result3"},
			},
		},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewGraphFromConfig: %v", err)
	}

	pool := newMockGPool()
	eng, err := NewEngine(g, pool)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	ctx := context.Background()
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	defer eng.Close(ctx)

	if eng.VertexSkipped("merge") {
		t.Fatal("expected merge to run (ai branch ran)")
	}
	val, ok := eng.GetOutput("final_result3")
	if !ok {
		t.Fatal("GetOutput(final_result3) not found")
	}
	result := *val.(*string)
	if result != "ai:sydney" {
		t.Errorf("expected %q, got %q", "ai:sydney", result)
	}
}
