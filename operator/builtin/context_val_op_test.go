package builtin

import (
	"context"
	"encoding/json"
	"testing"

	dagor "github.com/akennis/dagor"
	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/operator"
)

type cvTestIntKey struct{}
type cvTestStrKey struct{}

var (
	cvIntKey = cvTestIntKey{}
	cvStrKey = cvTestStrKey{}
)

func init() {
	_ = operator.RegisterOpFactory("_cvTestIntOp", ContextValFactory[int](cvTestIntKey{}))
	_ = operator.RegisterOpFactory("_cvTestStrOp", ContextValFactory[string](cvTestStrKey{}))
}

// ─── Run: happy paths ─────────────────────────────────────────────────────────

func TestContextValOp_Run_Int(t *testing.T) {
	op := &ContextValOp[int]{key: cvIntKey}
	ctx := context.WithValue(context.Background(), cvIntKey, 42)
	if err := op.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if op.Result != 42 {
		t.Errorf("expected 42, got %d", op.Result)
	}
}

func TestContextValOp_Run_String(t *testing.T) {
	op := &ContextValOp[string]{key: cvStrKey}
	ctx := context.WithValue(context.Background(), cvStrKey, "hello")
	if err := op.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if op.Result != "hello" {
		t.Errorf("expected %q, got %q", "hello", op.Result)
	}
}

// ─── Run: error cases ─────────────────────────────────────────────────────────

func TestContextValOp_Run_MissingKey(t *testing.T) {
	op := &ContextValOp[int]{key: cvIntKey}
	if err := op.Run(context.Background()); err == nil {
		t.Fatal("expected error for missing context key")
	}
}

func TestContextValOp_Run_WrongType(t *testing.T) {
	op := &ContextValOp[int]{key: cvIntKey}
	ctx := context.WithValue(context.Background(), cvIntKey, "not-an-int")
	if err := op.Run(ctx); err == nil {
		t.Fatal("expected error for wrong value type")
	}
}

// ─── Setup / Reset ────────────────────────────────────────────────────────────

func TestContextValOp_Setup_Reset(t *testing.T) {
	op := &ContextValOp[int]{key: cvIntKey}
	if err := op.Setup(mustParams(t, `{}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := op.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
}

// ─── Fields ───────────────────────────────────────────────────────────────────

func TestContextValOp_ResetFields(t *testing.T) {
	op := &ContextValOp[int]{key: cvIntKey}
	op.Result = 99
	op.ResetFields()
	if op.Result != 0 {
		t.Errorf("expected 0 after ResetFields, got %d", op.Result)
	}
}

func TestContextValOp_SetInputField_AlwaysErrors(t *testing.T) {
	op := &ContextValOp[string]{key: cvStrKey}
	if err := op.SetInputField("anything", "value"); err == nil {
		t.Error("expected error from SetInputField")
	}
}

func TestContextValOp_InputFields_Empty(t *testing.T) {
	op := &ContextValOp[int]{key: cvIntKey}
	if len(op.InputFields()) != 0 {
		t.Error("expected empty InputFields")
	}
}

// ─── Graph reuse ─────────────────────────────────────────────────────────────
//
// The *graph.Graph is built once. A fresh Engine is created per call (the
// engine holds one-shot execution state; the graph holds only topology), so
// the same graph definition serves many executions with different context
// values — the key pattern for request pipelines and servers.

func TestContextValOp_GraphReuse(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{})
	cfg := &config.GraphConfig{
		Name: "cv_reuse_test",
		Vertices: map[string]*config.VertexConfig{
			"emit": {
				Op:      "_cvTestIntOp",
				Params:  raw,
				Inputs:  map[string]string{},
				Outputs: map[string]string{"Result": "out"},
			},
		},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewGraphFromConfig: %v", err)
	}

	run := func(want int) {
		t.Helper()
		eng, err := dagor.NewEngine(g, simplePool{})
		if err != nil {
			t.Fatalf("NewEngine: %v", err)
		}
		ctx := context.WithValue(context.Background(), cvIntKey, want)
		defer eng.Close(ctx)
		if err := eng.Run(ctx); err != nil {
			t.Fatalf("Run(%d): %v", want, err)
		}
		val, ok := eng.GetOutput("out")
		if !ok {
			t.Fatalf("GetOutput(out) not found for want=%d", want)
		}
		if got := *val.(*int); got != want {
			t.Errorf("want %d, got %d", want, got)
		}
	}

	run(3)
	run(99)
	run(-1)
}

// ─── DAG round-trip ───────────────────────────────────────────────────────────

func TestContextValOp_DAGRoundTrip(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{})
	cfg := &config.GraphConfig{
		Name: "cv_dag_test",
		Vertices: map[string]*config.VertexConfig{
			"emit": {
				Op:      "_cvTestIntOp",
				Params:  raw,
				Inputs:  map[string]string{},
				Outputs: map[string]string{"Result": "out"},
			},
		},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewGraphFromConfig: %v", err)
	}
	eng, err := dagor.NewEngine(g, simplePool{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	ctx := context.WithValue(context.Background(), cvIntKey, 7)
	defer eng.Close(ctx)
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	val, ok := eng.GetOutput("out")
	if !ok {
		t.Fatal("GetOutput(out) not found")
	}
	if result := *val.(*int); result != 7 {
		t.Errorf("expected 7, got %d", result)
	}
}
