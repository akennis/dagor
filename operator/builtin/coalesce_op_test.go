package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/akennis/dagor/config"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func mustParams(t *testing.T, raw string) *config.Params {
	t.Helper()
	p, err := config.NewFromRaw(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("NewFromRaw(%s): %v", raw, err)
	}
	return p
}

// ─── CoalesceOp[string] ───────────────────────────────────────────────────────

func TestCoalesceOp_Setup_Reset(t *testing.T) {
	op := &CoalesceOp[string]{}
	if err := op.Setup(mustParams(t, `{}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := op.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
}

func TestCoalesceOp_InputOutputFields(t *testing.T) {
	op := &CoalesceOp[string]{}
	ins := op.InputFields()
	if _, ok := ins["A"]; !ok {
		t.Error("expected input field A")
	}
	if _, ok := ins["B"]; !ok {
		t.Error("expected input field B")
	}
	outs := op.OutputFields()
	if _, ok := outs["Result"]; !ok {
		t.Error("expected output field Result")
	}
}

func TestCoalesceOp_Run_AOnly(t *testing.T) {
	op := &CoalesceOp[string]{}
	s := "from-a"
	op.A = &s
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.Result != "from-a" {
		t.Errorf("expected %q, got %q", "from-a", op.Result)
	}
}

func TestCoalesceOp_Run_BOnly(t *testing.T) {
	op := &CoalesceOp[string]{}
	s := "from-b"
	op.B = &s
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.Result != "from-b" {
		t.Errorf("expected %q, got %q", "from-b", op.Result)
	}
}

func TestCoalesceOp_Run_BothSet_PrefersA(t *testing.T) {
	op := &CoalesceOp[string]{}
	a, b := "first", "second"
	op.A = &a
	op.B = &b
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.Result != "first" {
		t.Errorf("expected A wins=%q, got %q", "first", op.Result)
	}
}

func TestCoalesceOp_Run_AllNil_Error(t *testing.T) {
	op := &CoalesceOp[string]{}
	if err := op.Run(context.Background()); err == nil {
		t.Error("expected error when all inputs are nil")
	}
}

func TestCoalesceOp_SetInputField_A(t *testing.T) {
	op := &CoalesceOp[string]{}
	s := "alpha"
	if err := op.SetInputField("A", &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.A == nil || *op.A != "alpha" {
		t.Errorf("expected A=%q", "alpha")
	}
}

func TestCoalesceOp_SetInputField_B(t *testing.T) {
	op := &CoalesceOp[string]{}
	s := "beta"
	if err := op.SetInputField("B", &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.B == nil || *op.B != "beta" {
		t.Errorf("expected B=%q", "beta")
	}
}

func TestCoalesceOp_SetInputField_Nil_IsNoOp(t *testing.T) {
	op := &CoalesceOp[string]{}
	// nil means "skipped branch" — field retains its nil zero value.
	if err := op.SetInputField("A", nil); err != nil {
		t.Fatalf("unexpected error on nil: %v", err)
	}
	if op.A != nil {
		t.Error("expected A to remain nil after SetInputField(nil)")
	}
}

func TestCoalesceOp_SetInputField_WrongType(t *testing.T) {
	op := &CoalesceOp[string]{}
	n := 42
	if err := op.SetInputField("A", &n); err == nil {
		t.Error("expected type mismatch error")
	}
}

func TestCoalesceOp_SetInputField_WrongType_ErrorMessage(t *testing.T) {
	// BUG-04: error must print "*string" not "**string"
	opA := &CoalesceOp[string]{}
	n := 42
	err := opA.SetInputField("A", &n)
	if err == nil {
		t.Fatal("expected error for field A")
	}
	msg := err.Error()
	if strings.Contains(msg, "**") {
		t.Errorf("field A error contains double asterisk: %q", msg)
	}
	if !strings.Contains(msg, "*string") {
		t.Errorf("field A error missing '*string': %q", msg)
	}

	opB := &CoalesceOp[string]{}
	err = opB.SetInputField("B", &n)
	if err == nil {
		t.Fatal("expected error for field B")
	}
	msg = err.Error()
	if strings.Contains(msg, "**") {
		t.Errorf("field B error contains double asterisk: %q", msg)
	}
	if !strings.Contains(msg, "*string") {
		t.Errorf("field B error missing '*string': %q", msg)
	}
}

func TestCoalesceOp_SetInputField_UnknownField(t *testing.T) {
	op := &CoalesceOp[string]{}
	s := "x"
	if err := op.SetInputField("Z", &s); err == nil {
		t.Error("expected error for unknown field")
	}
}

func TestCoalesceOp_ResetFields(t *testing.T) {
	op := &CoalesceOp[string]{}
	a, b := "a", "b"
	op.A = &a
	op.B = &b
	op.Result = "r"
	op.ResetFields()
	if op.A != nil {
		t.Error("expected A nil after reset")
	}
	if op.B != nil {
		t.Error("expected B nil after reset")
	}
	if op.Result != "" {
		t.Errorf("expected Result zero after reset, got %q", op.Result)
	}
}

func TestCoalesceOp_Int(t *testing.T) {
	op := &CoalesceOp[int]{}
	n := 42
	op.B = &n
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.Result != 42 {
		t.Errorf("expected 42, got %d", op.Result)
	}
}

// ─── CoalesceNOp[string] ──────────────────────────────────────────────────────

func TestCoalesceNOp_Setup_InvalidN(t *testing.T) {
	cases := []struct {
		n   int
		bad bool
	}{
		{0, true},
		{1, true},
		{2, false},
		{5, false},
	}
	for _, tc := range cases {
		op := &CoalesceNOp[string]{}
		raw := fmt.Sprintf(`{"n":%d}`, tc.n)
		err := op.Setup(mustParams(t, raw))
		if tc.bad && err == nil {
			t.Errorf("n=%d: expected error", tc.n)
		}
		if !tc.bad && err != nil {
			t.Errorf("n=%d: unexpected error: %v", tc.n, err)
		}
	}
}

func TestCoalesceNOp_Setup_MissingN(t *testing.T) {
	op := &CoalesceNOp[string]{}
	err := op.Setup(mustParams(t, `{}`))
	if err == nil {
		t.Fatal("expected error when n is absent")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error message, got: %q", err.Error())
	}
}

func TestCoalesceNOp_Setup_WrongTypeString(t *testing.T) {
	op := &CoalesceNOp[string]{}
	err := op.Setup(mustParams(t, `{"n":"three"}`))
	if err == nil {
		t.Fatal("expected error when n is a string")
	}
	msg := err.Error()
	if !strings.Contains(msg, "string") {
		t.Errorf("expected 'string' in error message for string n, got: %q", msg)
	}
	if strings.Contains(msg, "got 0") {
		t.Errorf("misleading 'got 0' in error message for string n: %q", msg)
	}
}

func TestCoalesceNOp_Setup_ErrorMessages_Distinct(t *testing.T) {
	// missing n
	opMissing := &CoalesceNOp[string]{}
	errMissing := opMissing.Setup(mustParams(t, `{}`))
	if errMissing == nil {
		t.Fatal("expected error for missing n")
	}

	// string n
	opString := &CoalesceNOp[string]{}
	errString := opString.Setup(mustParams(t, `{"n":"bad"}`))
	if errString == nil {
		t.Fatal("expected error for string n")
	}

	// integer n < 2
	opSmall := &CoalesceNOp[string]{}
	errSmall := opSmall.Setup(mustParams(t, `{"n":1}`))
	if errSmall == nil {
		t.Fatal("expected error for n=1")
	}

	// all three must be different messages
	if errMissing.Error() == errString.Error() {
		t.Errorf("missing and string errors should differ:\n  missing: %q\n  string:  %q", errMissing, errString)
	}
	if errMissing.Error() == errSmall.Error() {
		t.Errorf("missing and small-int errors should differ:\n  missing: %q\n  small:   %q", errMissing, errSmall)
	}
	if errString.Error() == errSmall.Error() {
		t.Errorf("string and small-int errors should differ:\n  string: %q\n  small:  %q", errString, errSmall)
	}
}

func TestCoalesceNOp_InputFields(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":3}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	ins := op.InputFields()
	for _, name := range []string{"Input0", "Input1", "Input2"} {
		if _, ok := ins[name]; !ok {
			t.Errorf("expected input field %s", name)
		}
	}
	if len(ins) != 3 {
		t.Errorf("expected 3 input fields, got %d", len(ins))
	}
}

func TestCoalesceNOp_OutputField(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":2}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if _, ok := op.OutputFields()["Result"]; !ok {
		t.Error("expected output field Result")
	}
}

func TestCoalesceNOp_Run_FirstInput(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":3}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	s := "zero"
	if err := op.SetInputField("Input0", &s); err != nil {
		t.Fatalf("SetInputField: %v", err)
	}
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	outs := op.OutputFields()
	result := *outs["Result"].(*string)
	if result != "zero" {
		t.Errorf("expected %q, got %q", "zero", result)
	}
}

func TestCoalesceNOp_Run_MiddleInput(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":3}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	s := "middle"
	if err := op.SetInputField("Input1", &s); err != nil {
		t.Fatalf("SetInputField: %v", err)
	}
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	outs := op.OutputFields()
	result := *outs["Result"].(*string)
	if result != "middle" {
		t.Errorf("expected %q, got %q", "middle", result)
	}
}

func TestCoalesceNOp_Run_LastInput(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":3}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	s := "last"
	if err := op.SetInputField("Input2", &s); err != nil {
		t.Fatalf("SetInputField: %v", err)
	}
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	outs := op.OutputFields()
	result := *outs["Result"].(*string)
	if result != "last" {
		t.Errorf("expected %q, got %q", "last", result)
	}
}

func TestCoalesceNOp_Run_AllNil_Error(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":2}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := op.Run(context.Background()); err == nil {
		t.Error("expected error when all inputs are nil")
	}
}

func TestCoalesceNOp_Run_FirstWins(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":3}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	a, b := "a", "b"
	_ = op.SetInputField("Input0", &a)
	_ = op.SetInputField("Input2", &b)
	if err := op.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	outs := op.OutputFields()
	result := *outs["Result"].(*string)
	if result != "a" {
		t.Errorf("expected first non-nil wins=%q, got %q", "a", result)
	}
}

func TestCoalesceNOp_SetInputField_UnknownName(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":2}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	s := "x"
	if err := op.SetInputField("BadName", &s); err == nil {
		t.Error("expected error for invalid field name")
	}
}

func TestCoalesceNOp_SetInputField_OutOfRange(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":2}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	s := "x"
	if err := op.SetInputField("Input5", &s); err == nil {
		t.Error("expected error for out-of-range index")
	}
}

func TestCoalesceNOp_SetInputField_WrongType(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":2}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	n := 99
	if err := op.SetInputField("Input0", &n); err == nil {
		t.Error("expected type mismatch error")
	}
}

func TestCoalesceNOp_SetInputField_Nil(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":2}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	s := "before"
	_ = op.SetInputField("Input0", &s)
	if err := op.SetInputField("Input0", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After nil, that slot is cleared to nil.
	if op.inputs[0] != nil {
		t.Error("expected Input0 to be nil after SetInputField(nil)")
	}
}

func TestCoalesceNOp_ResetFields(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":2}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	s := "val"
	_ = op.SetInputField("Input0", &s)
	_ = op.Run(context.Background())
	op.ResetFields()
	for i, inp := range op.inputs {
		if inp != nil {
			t.Errorf("expected Input%d nil after reset", i)
		}
	}
	outs := op.OutputFields()
	if result := *outs["Result"].(*string); result != "" {
		t.Errorf("expected Result zero after reset, got %q", result)
	}
}

func TestCoalesceNOp_Reset_ZerosNAndInputs(t *testing.T) {
	op := &CoalesceNOp[string]{}
	if err := op.Setup(mustParams(t, `{"n":3}`)); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if op.n != 3 || op.inputs == nil {
		t.Fatal("pre-condition: expected n=3 and inputs non-nil after Setup")
	}
	if err := op.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if op.n != 0 {
		t.Errorf("expected n=0 after Reset, got %d", op.n)
	}
	if op.inputs != nil {
		t.Errorf("expected inputs=nil after Reset, got len=%d", len(op.inputs))
	}
}
