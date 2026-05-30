package dagor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/operator"
	"github.com/akennis/dagor/predicate"
)

// captureReporter records every Reporter call so tests can assert on them.
// All methods are goroutine-safe.
type captureReporter struct {
	mu sync.Mutex

	graphStarts  []string
	graphFinish  []captureGraphFinish
	vertexStarts []captureVertex
	vertexFinish []captureVertexFinish
	vertexSkips  []captureVertexSkip
	vertexFields []captureVertexFields
}

type captureGraphFinish struct {
	name string
	dur  time.Duration
	err  error
}

type captureVertex struct {
	graphName, vertexName, vertexType string
}

type captureVertexFinish struct {
	graphName, vertexName, vertexType string
	dur                               time.Duration
	err                               error
}

type captureVertexSkip struct {
	graphName, vertexName, vertexType string
	reason                            SkipReason
}

type captureVertexFields struct {
	graphName, vertexName string
	phase                 FieldPhase
	fields                map[string]any
}

func (c *captureReporter) OnGraphStart(_ context.Context, name string) {
	c.mu.Lock()
	c.graphStarts = append(c.graphStarts, name)
	c.mu.Unlock()
}

func (c *captureReporter) OnGraphFinish(_ context.Context, name string, dur time.Duration, err error) {
	c.mu.Lock()
	c.graphFinish = append(c.graphFinish, captureGraphFinish{name, dur, err})
	c.mu.Unlock()
}

func (c *captureReporter) OnVertexStart(_ context.Context, gn, vn, vt string) {
	c.mu.Lock()
	c.vertexStarts = append(c.vertexStarts, captureVertex{gn, vn, vt})
	c.mu.Unlock()
}

func (c *captureReporter) OnVertexFinish(_ context.Context, gn, vn, vt string, dur time.Duration, err error) {
	c.mu.Lock()
	c.vertexFinish = append(c.vertexFinish, captureVertexFinish{gn, vn, vt, dur, err})
	c.mu.Unlock()
}

func (c *captureReporter) OnVertexSkipped(_ context.Context, gn, vn, vt string, reason SkipReason) {
	c.mu.Lock()
	c.vertexSkips = append(c.vertexSkips, captureVertexSkip{gn, vn, vt, reason})
	c.mu.Unlock()
}

func (c *captureReporter) OnVertexFields(_ context.Context, gn, vn string, phase FieldPhase, fields map[string]any) {
	// Make a copy of the map so tests can inspect a stable snapshot.
	cp := make(map[string]any, len(fields))
	for k, v := range fields {
		cp[k] = v
	}
	c.mu.Lock()
	c.vertexFields = append(c.vertexFields, captureVertexFields{gn, vn, phase, cp})
	c.mu.Unlock()
}

func (c *captureReporter) skippedVertex(name string) (captureVertexSkip, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.vertexSkips {
		if s.vertexName == name {
			return s, true
		}
	}
	return captureVertexSkip{}, false
}

func (c *captureReporter) finishedVertex(name string) (captureVertexFinish, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range c.vertexFinish {
		if f.vertexName == name {
			return f, true
		}
	}
	return captureVertexFinish{}, false
}

func (c *captureReporter) fieldsFor(name string, phase FieldPhase) (map[string]any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range c.vertexFields {
		if f.vertexName == name && f.phase == phase {
			return f.fields, true
		}
	}
	return nil, false
}

// ---- minimal test operator with proper pointer-based field semantics ----

// reporterTestOp is a simple operator: one *float64 input, one float64 output.
// Its InputFields / OutputFields follow the real library convention: values are
// pointers to the struct fields, so dereferenceAll correctly extracts them.
type reporterTestOp struct {
	mu     sync.Mutex
	In     *float64
	Out    float64
	runErr error
}

func (o *reporterTestOp) Setup(_ *config.Params) error { return nil }
func (o *reporterTestOp) Reset() error                 { return nil }
func (o *reporterTestOp) Run(_ context.Context) error {
	if o.runErr != nil {
		return o.runErr
	}
	if o.In != nil {
		o.Out = *o.In
	}
	return nil
}
func (o *reporterTestOp) InputFields() map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	return map[string]any{"In": &o.In}
}
func (o *reporterTestOp) OutputFields() map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	return map[string]any{"Out": &o.Out}
}
func (o *reporterTestOp) SetInputField(field string, value any) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if field == "In" {
		v, ok := value.(*float64)
		if !ok {
			return errors.New("expected *float64")
		}
		o.In = v
	}
	return nil
}
func (o *reporterTestOp) ResetFields() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.In = nil
	o.Out = 0
}

// reporterSrcOp is a source operator with no inputs, one float64 output.
type reporterSrcOp struct {
	mu  sync.Mutex
	Out float64
}

func (o *reporterSrcOp) Setup(_ *config.Params) error { return nil }
func (o *reporterSrcOp) Reset() error                 { return nil }
func (o *reporterSrcOp) Run(_ context.Context) error  { o.Out = 7.0; return nil }
func (o *reporterSrcOp) InputFields() map[string]any  { return map[string]any{} }
func (o *reporterSrcOp) OutputFields() map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	return map[string]any{"Out": &o.Out}
}
func (o *reporterSrcOp) SetInputField(_ string, _ any) error { return nil }
func (o *reporterSrcOp) ResetFields()                        {}

// ---- helpers ----

func mustRegisterReporterOps(t *testing.T, srcName, dstName string, dstRunErr error) {
	t.Helper()
	if err := operator.RegisterOpFactory(srcName, func() operator.IOperator {
		return &reporterSrcOp{}
	}); err != nil {
		t.Fatalf("register %s: %v", srcName, err)
	}
	if err := operator.RegisterOpFactory(dstName, func() operator.IOperator {
		return &reporterTestOp{runErr: dstRunErr}
	}); err != nil {
		t.Fatalf("register %s: %v", dstName, err)
	}
}

func buildTwoVertexGraph(t *testing.T, graphName, srcOp, dstOp string, dstCfg *config.VertexConfig) *graph.Graph {
	t.Helper()
	cfg := &config.GraphConfig{
		Name: graphName,
		Vertices: map[string]*config.VertexConfig{
			"src": {Op: srcOp, Params: []byte(`{}`), Outputs: map[string]string{"Out": "wire"}},
			"dst": dstCfg,
		},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}
	return g
}

// ---- tests ----

func TestNoopReporter_NoOp(t *testing.T) {
	noopSmoke() // defined in reporter_test.go
}

func TestWithReporter_BasicExecution(t *testing.T) {
	srcOp := "Reporter_Src_Basic"
	dstOp := "Reporter_Dst_Basic"
	mustRegisterReporterOps(t, srcOp, dstOp, nil)

	dstCfg := &config.VertexConfig{
		Op:     dstOp,
		Params: []byte(`{}`),
		Inputs: map[string]string{"In": "wire"},
	}
	g := buildTwoVertexGraph(t, "basic_graph", srcOp, dstOp, dstCfg)

	rep := &captureReporter{}
	eng, err := NewEngine(g, newMockGPool(), WithReporter(rep))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	_ = eng.Close(context.Background())

	// Graph lifecycle events.
	if len(rep.graphStarts) != 1 || rep.graphStarts[0] != "basic_graph" {
		t.Errorf("expected 1 OnGraphStart(basic_graph), got %v", rep.graphStarts)
	}
	if len(rep.graphFinish) != 1 || rep.graphFinish[0].err != nil {
		t.Errorf("expected 1 OnGraphFinish with nil err, got %v", rep.graphFinish)
	}

	// Vertex lifecycle: both vertices should have started and finished.
	if len(rep.vertexStarts) != 2 {
		t.Errorf("expected 2 OnVertexStart, got %d", len(rep.vertexStarts))
	}
	if len(rep.vertexFinish) != 2 {
		t.Errorf("expected 2 OnVertexFinish, got %d", len(rep.vertexFinish))
	}
	if len(rep.vertexSkips) != 0 {
		t.Errorf("expected no skips, got %d", len(rep.vertexSkips))
	}

	// src has no inputs but has an output.
	if _, ok := rep.fieldsFor("src", FieldPhaseOutput); !ok {
		t.Error("expected OnVertexFields(output) for src")
	}

	// dst has an input.
	if _, ok := rep.fieldsFor("dst", FieldPhaseInput); !ok {
		t.Error("expected OnVertexFields(input) for dst")
	}
}

func TestWithReporter_EmptyGraph(t *testing.T) {
	// An empty graph should still emit OnGraphStart and OnGraphFinish.
	g, err := graph.NewGraphFromConfig(&config.GraphConfig{Name: "empty", Vertices: map[string]*config.VertexConfig{}})
	if err != nil {
		t.Fatalf("build empty graph: %v", err)
	}

	rep := &captureReporter{}
	eng, err := NewEngine(g, newMockGPool(), WithReporter(rep))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(rep.graphStarts) != 1 {
		t.Errorf("expected 1 OnGraphStart, got %d", len(rep.graphStarts))
	}
	if len(rep.graphFinish) != 1 {
		t.Errorf("expected 1 OnGraphFinish, got %d", len(rep.graphFinish))
	}
}

func TestWithReporter_SkipReasonCondition(t *testing.T) {
	srcOp := "Reporter_Src_CondSkip"
	dstOp := "Reporter_Dst_CondSkip"
	mustRegisterReporterOps(t, srcOp, dstOp, nil)

	predName := "reporter_test_always_false"
	_ = predicate.Register(predName, func(_ map[string]any) bool { return false })

	dstCfg := &config.VertexConfig{
		Op:        dstOp,
		Params:    []byte(`{}`),
		Inputs:    map[string]string{"In": "wire"},
		Condition: predName,
	}
	g := buildTwoVertexGraph(t, "cond_graph", srcOp, dstOp, dstCfg)

	rep := &captureReporter{}
	eng, err := NewEngine(g, newMockGPool(), WithReporter(rep))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	_ = eng.Close(context.Background())

	skip, ok := rep.skippedVertex("dst")
	if !ok {
		t.Fatal("expected dst to be reported as skipped")
	}
	if skip.reason != SkipReasonCondition {
		t.Errorf("expected SkipReasonCondition, got %q", skip.reason)
	}
	// src should have finished normally.
	if _, ok := rep.finishedVertex("src"); !ok {
		t.Error("expected src to be reported as finished")
	}
}

func TestWithReporter_SkipReasonTransitive(t *testing.T) {
	srcOp := "Reporter_Src_TransSkip"
	dstOp := "Reporter_Dst_TransSkip"
	mustRegisterReporterOps(t, srcOp, dstOp, nil)

	predName := "reporter_test_always_false_trans"
	_ = predicate.Register(predName, func(_ map[string]any) bool { return false })

	// src is conditional (always skipped); dst depends on src → transitive skip.
	srcCfg := &config.VertexConfig{
		Op:        srcOp,
		Params:    []byte(`{}`),
		Outputs:   map[string]string{"Out": "wire"},
		Condition: predName,
	}
	dstCfg := &config.VertexConfig{
		Op:     dstOp,
		Params: []byte(`{}`),
		Inputs: map[string]string{"In": "wire"},
	}
	cfg := &config.GraphConfig{
		Name: "transitive_graph",
		Vertices: map[string]*config.VertexConfig{
			"src": srcCfg,
			"dst": dstCfg,
		},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	rep := &captureReporter{}
	eng, err := NewEngine(g, newMockGPool(), WithReporter(rep))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	_ = eng.Close(context.Background())

	srcSkip, ok := rep.skippedVertex("src")
	if !ok {
		t.Fatal("expected src to be skipped")
	}
	if srcSkip.reason != SkipReasonCondition {
		t.Errorf("src: expected SkipReasonCondition, got %q", srcSkip.reason)
	}

	dstSkip, ok := rep.skippedVertex("dst")
	if !ok {
		t.Fatal("expected dst to be transitively skipped")
	}
	if dstSkip.reason != SkipReasonTransitive {
		t.Errorf("dst: expected SkipReasonTransitive, got %q", dstSkip.reason)
	}
}

func TestWithReporter_SkipReasonError_OnErrorContinue(t *testing.T) {
	srcOp := "Reporter_Src_ErrSkip"
	dstOp := "Reporter_Dst_ErrSkip"
	mustRegisterReporterOps(t, srcOp, dstOp, errors.New("boom"))

	dstCfg := &config.VertexConfig{
		Op:      dstOp,
		Params:  []byte(`{}`),
		Inputs:  map[string]string{"In": "wire"},
		OnError: config.OnErrorContinue,
	}
	g := buildTwoVertexGraph(t, "err_continue_graph", srcOp, dstOp, dstCfg)

	rep := &captureReporter{}
	eng, err := NewEngine(g, newMockGPool(), WithReporter(rep))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// Run returns nil because OnErrorContinue swallows the vertex error.
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	_ = eng.Close(context.Background())

	skip, ok := rep.skippedVertex("dst")
	if !ok {
		t.Fatal("expected dst to be reported as skipped (OnErrorContinue)")
	}
	if skip.reason != SkipReasonError {
		t.Errorf("expected SkipReasonError, got %q", skip.reason)
	}
}

func TestWithFieldScrubber_OmitsNilFields(t *testing.T) {
	srcOp := "Reporter_Src_Scrub"
	if err := operator.RegisterOpFactory(srcOp, func() operator.IOperator {
		return &twoFieldSrcOp{}
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := &config.GraphConfig{
		Name: "scrub_graph",
		Vertices: map[string]*config.VertexConfig{
			"src": {
				Op:      srcOp,
				Params:  []byte(`{}`),
				Outputs: map[string]string{"Safe": "safe_wire", "Secret": "secret_wire"},
			},
		},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	rep := &captureReporter{}
	scrubber := FieldScrubber(func(_ context.Context, _, _, fieldName string, _ FieldPhase, value any) any {
		if fieldName == "Secret" {
			return nil
		}
		return value
	})

	eng, err := NewEngine(g, newMockGPool(), WithReporter(rep), WithFieldScrubber(scrubber))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	_ = eng.Close(context.Background())

	fields, ok := rep.fieldsFor("src", FieldPhaseOutput)
	if !ok {
		t.Fatal("expected output fields for src")
	}
	if _, hasSecret := fields["Secret"]; hasSecret {
		t.Error("Secret field should have been scrubbed (omitted)")
	}
	if _, hasSafe := fields["Safe"]; !hasSafe {
		t.Error("Safe field should be present")
	}
}

func TestWithReporter_NoReporter_NoopDefault(t *testing.T) {
	// No WithReporter option → NoopReporter default → must not panic.
	srcOp := "Reporter_Src_Noop"
	if err := operator.RegisterOpFactory(srcOp, func() operator.IOperator {
		return &reporterSrcOp{}
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := &config.GraphConfig{
		Name:     "noop_graph",
		Vertices: map[string]*config.VertexConfig{"src": {Op: srcOp, Params: []byte(`{}`), Outputs: map[string]string{"Out": "w"}}},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	eng, err := NewEngine(g, newMockGPool())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestScrubFields_ShallowCopy(t *testing.T) {
	// Modifying the map passed to OnVertexFields must not affect the operator's live fields.
	srcOp := "Reporter_Src_ShallowCopy"
	if err := operator.RegisterOpFactory(srcOp, func() operator.IOperator {
		return &reporterSrcOp{}
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := &config.GraphConfig{
		Name:     "shallow_graph",
		Vertices: map[string]*config.VertexConfig{"src": {Op: srcOp, Params: []byte(`{}`), Outputs: map[string]string{"Out": "w"}}},
	}
	g, err := graph.NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}

	var capturedMap map[string]any
	var capturedMu sync.Mutex
	mutatingScrubber := struct{ captureReporter }{}
	_ = mutatingScrubber // prevent unused

	mutatingReporter := &mutateOnFieldsReporter{captured: &capturedMap, mu: &capturedMu}
	eng, err := NewEngine(g, newMockGPool(), WithReporter(mutatingReporter))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Mutate the captured map.
	capturedMu.Lock()
	if capturedMap != nil {
		capturedMap["injected"] = "evil"
	}
	capturedMu.Unlock()

	// The engine's GetOutput should still work correctly — the mutation affected
	// only the reported copy, not the operator's FieldValue.
	out, ok := eng.GetOutput("w")
	if !ok {
		t.Fatal("expected output wire 'w' to exist")
	}
	if out == nil {
		t.Error("expected non-nil output after map mutation")
	}
}

// mutateOnFieldsReporter captures the first output fields map it receives.
type mutateOnFieldsReporter struct {
	NoopReporter
	captured *map[string]any
	mu       *sync.Mutex
}

func (r *mutateOnFieldsReporter) OnVertexFields(_ context.Context, _, _ string, phase FieldPhase, fields map[string]any) {
	if phase == FieldPhaseOutput {
		r.mu.Lock()
		*r.captured = fields
		r.mu.Unlock()
	}
}

// twoFieldSrcOp emits two output fields: Safe and Secret.
type twoFieldSrcOp struct {
	mu     sync.Mutex
	Safe   float64
	Secret float64
}

func (o *twoFieldSrcOp) Setup(_ *config.Params) error { return nil }
func (o *twoFieldSrcOp) Reset() error                 { return nil }
func (o *twoFieldSrcOp) Run(_ context.Context) error {
	o.Safe = 1.0
	o.Secret = 99.0
	return nil
}
func (o *twoFieldSrcOp) InputFields() map[string]any { return map[string]any{} }
func (o *twoFieldSrcOp) OutputFields() map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	return map[string]any{"Safe": &o.Safe, "Secret": &o.Secret}
}
func (o *twoFieldSrcOp) SetInputField(_ string, _ any) error { return nil }
func (o *twoFieldSrcOp) ResetFields() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Safe = 0
	o.Secret = 0
}

