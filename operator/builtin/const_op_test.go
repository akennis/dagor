package builtin

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	dagor "github.com/wwz16/dagor"
	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/graph"
	"github.com/wwz16/dagor/operator"
)

// simplePool is a minimal IGPool for const_op tests.
type simplePool struct{}

func (simplePool) Submit(fn func()) error { go fn(); return nil }
func (simplePool) Release()               {}

// ─── Four registered types: Setup/Run/OutputFields ───────────────────────────

func TestConstStringOp_RoundTrip(t *testing.T) {
	op, err := operator.GetOp("ConstStringOp")
	if err != nil {
		t.Fatalf("GetOp: %v", err)
	}
	if err := op.Setup(mustParams(t, `{"Value":"hello"}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	result := *op.OutputFields()["Result"].(*string)
	if result != "hello" {
		t.Errorf("expected %q, got %q", "hello", result)
	}
}

func TestConstIntOp_RoundTrip(t *testing.T) {
	op, err := operator.GetOp("ConstIntOp")
	if err != nil {
		t.Fatalf("GetOp: %v", err)
	}
	if err := op.Setup(mustParams(t, `{"Value":"42"}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	result := *op.OutputFields()["Result"].(*int)
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestConstFloat64Op_RoundTrip(t *testing.T) {
	op, err := operator.GetOp("ConstFloat64Op")
	if err != nil {
		t.Fatalf("GetOp: %v", err)
	}
	if err := op.Setup(mustParams(t, `{"Value":"3.14"}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	result := *op.OutputFields()["Result"].(*float64)
	if result != 3.14 {
		t.Errorf("expected 3.14, got %f", result)
	}
}

func TestConstBoolOp_RoundTrip(t *testing.T) {
	op, err := operator.GetOp("ConstBoolOp")
	if err != nil {
		t.Fatalf("GetOp: %v", err)
	}
	if err := op.Setup(mustParams(t, `{"Value":"true"}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	result := *op.OutputFields()["Result"].(*bool)
	if !result {
		t.Error("expected true")
	}
}

// ─── Invalid Value for numeric types ─────────────────────────────────────────

func TestConstIntOp_Setup_InvalidValue(t *testing.T) {
	op, err := operator.GetOp("ConstIntOp")
	if err != nil {
		t.Fatalf("GetOp: %v", err)
	}
	err = op.Setup(mustParams(t, `{"Value":"notanint"}`))
	if err == nil {
		t.Fatal("expected error for invalid Value")
	}
	if !strings.Contains(err.Error(), "notanint") {
		t.Errorf("expected bad value in error message, got: %q", err.Error())
	}
}

func TestConstFloat64Op_Setup_InvalidValue(t *testing.T) {
	op, err := operator.GetOp("ConstFloat64Op")
	if err != nil {
		t.Fatalf("GetOp: %v", err)
	}
	err = op.Setup(mustParams(t, `{"Value":"notafloat"}`))
	if err == nil {
		t.Fatal("expected error for invalid Value")
	}
	if !strings.Contains(err.Error(), "notafloat") {
		t.Errorf("expected bad value in error message, got: %q", err.Error())
	}
}

// ─── SetInputField always errors ──────────────────────────────────────────────

func TestConstOp_SetInputField_AlwaysErrors(t *testing.T) {
	op := &ConstOp[string]{parse: func(s string) (string, error) { return s, nil }}
	if err := op.SetInputField("anything", "value"); err == nil {
		t.Error("expected error from SetInputField")
	}
}

// ─── ResetFields zeros Result ────────────────────────────────────────────────

func TestConstOp_ResetFields(t *testing.T) {
	op := &ConstOp[int]{parse: func(s string) (int, error) { return strconv.Atoi(s) }}
	op.Result = 99
	op.ResetFields()
	if op.Result != 0 {
		t.Errorf("expected 0 after ResetFields, got %d", op.Result)
	}
}

// ─── Setup with nil parse errors ─────────────────────────────────────────────

func TestConstOp_Setup_NilParse_Errors(t *testing.T) {
	op := &ConstOp[string]{}
	err := op.Setup(mustParams(t, `{"Value":"x"}`))
	if err == nil {
		t.Fatal("expected error when parse is nil")
	}
}

// ─── DAG round-trip ───────────────────────────────────────────────────────────

func TestConstIntOp_DAGRoundTrip(t *testing.T) {
	params, _ := json.Marshal(map[string]string{"Value": "42"})
	cfg := &config.GraphConfig{
		Name: "const_dag_test",
		Vertices: map[string]*config.VertexConfig{
			"emit": {
				Op:      "ConstIntOp",
				Params:  params,
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
	ctx := context.Background()
	defer eng.Close(ctx)
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	val, ok := eng.GetOutput("out")
	if !ok {
		t.Fatal("GetOutput(out) not found")
	}
	if result := *val.(*int); result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}
