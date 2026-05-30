package reporter_test

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/akennis/dagor"
	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/operator"
	"github.com/akennis/dagor/reporter"
)

func newTestReporter(buf *strings.Builder) *reporter.SlogReporter {
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return reporter.New(logger)
}

func assertContains(t *testing.T, buf *strings.Builder, want string) {
	t.Helper()
	if !strings.Contains(buf.String(), want) {
		t.Errorf("expected log to contain %q\nactual log:\n%s", want, buf.String())
	}
}

func assertNotContains(t *testing.T, buf *strings.Builder, want string) {
	t.Helper()
	if strings.Contains(buf.String(), want) {
		t.Errorf("expected log NOT to contain %q\nactual log:\n%s", want, buf.String())
	}
}

func TestSlogReporter_OnGraphStart(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	r.OnGraphStart(context.Background(), "myGraph")
	assertContains(t, &buf, "graph.start")
	assertContains(t, &buf, "myGraph")
}

func TestSlogReporter_OnGraphStart_NoRunID_WhenContextIsPlain(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	r.OnGraphStart(context.Background(), "g")
	// Plain context has no run ID — run_id attribute should be absent.
	assertNotContains(t, &buf, "run_id=")
}

func TestSlogReporter_OnGraphFinish_NoError(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	r.OnGraphFinish(context.Background(), "myGraph", 42*time.Millisecond, nil)
	assertContains(t, &buf, "graph.finish")
	assertContains(t, &buf, "myGraph")
	assertContains(t, &buf, "dur_ms=42")
	if strings.Contains(buf.String(), "ERROR") {
		t.Error("expected INFO level for successful finish, got ERROR")
	}
}

func TestSlogReporter_OnGraphFinish_WithError(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	r.OnGraphFinish(context.Background(), "myGraph", time.Second, &testErr{"something failed"})
	assertContains(t, &buf, "graph.finish")
	assertContains(t, &buf, "ERROR")
	assertContains(t, &buf, "something failed")
}

func TestSlogReporter_OnVertexStart(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	r.OnVertexStart(context.Background(), "g", "myVertex", "op")
	assertContains(t, &buf, "vertex.start")
	assertContains(t, &buf, "myVertex")
	assertContains(t, &buf, "type=op")
}

func TestSlogReporter_OnVertexFinish_NoError(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	r.OnVertexFinish(context.Background(), "g", "v", "op", 5*time.Millisecond, nil)
	assertContains(t, &buf, "vertex.finish")
	assertContains(t, &buf, "dur_ms=5")
	if strings.Contains(buf.String(), "ERROR") {
		t.Error("expected DEBUG level for successful finish, got ERROR")
	}
}

func TestSlogReporter_OnVertexFinish_WithError(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	r.OnVertexFinish(context.Background(), "g", "v", "op", time.Millisecond, &testErr{"vertex boom"})
	assertContains(t, &buf, "vertex.finish")
	assertContains(t, &buf, "ERROR")
	assertContains(t, &buf, "vertex boom")
}

func TestSlogReporter_OnVertexSkipped(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	r.OnVertexSkipped(context.Background(), "g", "v", "op", dagor.SkipReasonCondition)
	assertContains(t, &buf, "vertex.skipped")
	assertContains(t, &buf, "reason=condition")
}

func TestSlogReporter_OnVertexFields(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	fields := map[string]any{"Temp": 36.6, "Label": "warm"}
	r.OnVertexFields(context.Background(), "g", "v", dagor.FieldPhaseInput, fields)
	assertContains(t, &buf, "vertex.fields")
	assertContains(t, &buf, "phase=input")
	assertContains(t, &buf, "Temp=")
	assertContains(t, &buf, "Label=warm")
}

func TestSlogReporter_OnVertexFields_OutputPhase(t *testing.T) {
	var buf strings.Builder
	r := newTestReporter(&buf)
	r.OnVertexFields(context.Background(), "g", "v", dagor.FieldPhaseOutput, map[string]any{"Result": 42})
	assertContains(t, &buf, "phase=output")
}

func TestSlogReporter_ImplementsReporter(t *testing.T) {
	var _ dagor.Reporter = reporter.New(slog.Default())
}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }

// ---- integration test: run_id present on every SlogReporter line ----

// integSrcOp is a minimal source operator for the integration test.
type integSrcOp struct {
	mu  sync.Mutex
	Out float64
}

func (o *integSrcOp) Setup(_ *config.Params) error { return nil }
func (o *integSrcOp) Reset() error                 { return nil }
func (o *integSrcOp) Run(_ context.Context) error  { o.Out = 1.0; return nil }
func (o *integSrcOp) InputFields() map[string]any  { return map[string]any{} }
func (o *integSrcOp) OutputFields() map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	return map[string]any{"Out": &o.Out}
}
func (o *integSrcOp) SetInputField(_ string, _ any) error { return nil }
func (o *integSrcOp) ResetFields()                        { o.mu.Lock(); o.Out = 0; o.mu.Unlock() }

func TestSlogReporter_RunIDPresentOnEveryLine(t *testing.T) {
	opName := "SlogInteg_Src_RunID"
	if err := operator.RegisterOpFactory(opName, func() operator.IOperator { return &integSrcOp{} }); err != nil {
		t.Fatalf("register op: %v", err)
	}

	g, err := graph.NewGraphFromConfig(&config.GraphConfig{
		Name: "integ_runid_graph",
		Vertices: map[string]*config.VertexConfig{
			"src": {Op: opName, Params: []byte(`{}`), Outputs: map[string]string{"Out": "w"}},
		},
	})
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	pool, err := ants.NewPool(2)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Release()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	eng, err := dagor.NewEngine(g, pool, dagor.WithReporter(reporter.New(logger)))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output == "" {
		t.Fatal("expected log output, got none")
	}
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(line, "run_id=") {
			t.Errorf("log line missing run_id:\n  %s", line)
		}
	}
}
