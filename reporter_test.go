package dagor

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/operator"
)

// compile-time interface check.
var _ Reporter = NoopReporter{}

// noopSmoke exercises all NoopReporter methods to confirm they don't panic.
func noopSmoke() {
	ctx := context.Background()
	n := NoopReporter{}
	n.OnGraphStart(ctx, "g")
	n.OnGraphFinish(ctx, "g", time.Second, nil)
	n.OnVertexStart(ctx, "g", "v", "op")
	n.OnVertexFinish(ctx, "g", "v", "op", time.Millisecond, nil)
	n.OnVertexSkipped(ctx, "g", "v", "op", SkipReasonCondition)
	n.OnVertexFields(ctx, "g", "v", FieldPhaseInput, map[string]any{"x": 1})
}

func TestRunID_EmptyOnPlainContext(t *testing.T) {
	if id := RunID(context.Background()); id != "" {
		t.Errorf("expected empty run ID on plain context, got %q", id)
	}
}

func TestRunID_PresentAfterRun(t *testing.T) {
	var capturedID string
	rep := &graphStartReporter{fn: func(ctx context.Context) { capturedID = RunID(ctx) }}

	opName := "RunID_Src_Present"
	_ = operator.RegisterOpFactory(opName, func() operator.IOperator { return &reporterSrcOp{} })

	cfg := &config.GraphConfig{
		Name:     "runid_graph",
		Vertices: map[string]*config.VertexConfig{"src": {Op: opName, Params: []byte(`{}`), Outputs: map[string]string{"Out": "w"}}},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}
	eng, err := NewEngine(g, newMockGPool(), WithReporter(rep))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if capturedID == "" {
		t.Fatal("expected non-empty run ID in reporter context")
	}
	// UUID v4 format: 8-4-4-4-12 hex chars separated by dashes.
	parts := strings.Split(capturedID, "-")
	if len(parts) != 5 {
		t.Errorf("run ID %q does not look like a UUID", capturedID)
	}
}

func TestRunID_UniquePerRun(t *testing.T) {
	var mu sync.Mutex
	var ids []string
	rep := &graphStartReporter{fn: func(ctx context.Context) {
		mu.Lock()
		ids = append(ids, RunID(ctx))
		mu.Unlock()
	}}

	opName := "RunID_Src_Unique"
	_ = operator.RegisterOpFactory(opName, func() operator.IOperator { return &reporterSrcOp{} })

	cfg := &config.GraphConfig{
		Name:     "runid_unique_graph",
		Vertices: map[string]*config.VertexConfig{"src": {Op: opName, Params: []byte(`{}`), Outputs: map[string]string{"Out": "w"}}},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	for range 3 {
		eng, err := NewEngine(g, newMockGPool(), WithReporter(rep))
		if err != nil {
			t.Fatalf("NewEngine: %v", err)
		}
		if err := eng.Run(context.Background()); err != nil {
			t.Fatalf("Run: %v", err)
		}
	}

	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate run ID %q across runs", id)
		}
		seen[id] = true
	}
	if len(seen) != 3 {
		t.Errorf("expected 3 distinct run IDs, got %d: %v", len(seen), ids)
	}
}

// graphStartReporter calls fn on OnGraphStart and is a no-op elsewhere.
type graphStartReporter struct {
	NoopReporter
	fn func(ctx context.Context)
}

func (r *graphStartReporter) OnGraphStart(ctx context.Context, _ string) { r.fn(ctx) }
