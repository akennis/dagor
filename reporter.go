package dagor

import (
	"context"
	"time"
)

// FieldPhase indicates whether a field report is for operator inputs or outputs.
type FieldPhase int

const (
	FieldPhaseInput  FieldPhase = iota
	FieldPhaseOutput
)

// SkipReason indicates why a vertex was skipped during execution.
type SkipReason string

const (
	// SkipReasonCondition means the vertex's own condition predicate returned false.
	SkipReasonCondition SkipReason = "condition"
	// SkipReasonTransitive means at least one predecessor vertex was skipped.
	SkipReasonTransitive SkipReason = "transitive"
	// SkipReasonError means the vertex (or condition evaluation) errored and OnError=continue.
	SkipReasonError SkipReason = "error"
)

// FieldScrubber is called for each operator field value before it is passed to Reporter.
// Parameters provide full context: graph name, vertex name, field name, phase (input or output),
// and the fully-dereferenced runtime value. Return nil to omit the field from the report.
type FieldScrubber func(ctx context.Context, graphName, vertexName, fieldName string,
	phase FieldPhase, value any) any

// Reporter is a goroutine-safe observability hook for workflow execution.
// All methods may be called concurrently from multiple goroutines.
//
// Event contract:
//   - OnGraphStart fires before any vertex events for a given Run.
//   - OnGraphFinish fires after all vertex events have completed.
//   - OnVertexStart fires for every vertex, including those that will be skipped.
//   - OnVertexFinish fires only when the vertex actually executed (never when skipped).
//   - OnVertexSkipped fires instead of OnVertexFinish when the vertex was skipped.
//   - OnVertexFields fires for Op vertices only: once for inputs (before Run),
//     once for outputs (after a successful Run). Map/Filter/Reduce vertices do not emit this event.
type Reporter interface {
	OnGraphStart(ctx context.Context, name string)
	OnGraphFinish(ctx context.Context, name string, dur time.Duration, err error)
	OnVertexStart(ctx context.Context, graphName, vertexName, vertexType string)
	OnVertexFinish(ctx context.Context, graphName, vertexName, vertexType string, dur time.Duration, err error)
	OnVertexSkipped(ctx context.Context, graphName, vertexName, vertexType string, reason SkipReason)
	OnVertexFields(ctx context.Context, graphName, vertexName string, phase FieldPhase, fields map[string]any)
}

// NoopReporter implements Reporter with empty methods. It is the default when no Reporter is configured.
type NoopReporter struct{}

func (NoopReporter) OnGraphStart(_ context.Context, _ string)                                       {}
func (NoopReporter) OnGraphFinish(_ context.Context, _ string, _ time.Duration, _ error)            {}
func (NoopReporter) OnVertexStart(_ context.Context, _, _, _ string)                                {}
func (NoopReporter) OnVertexFinish(_ context.Context, _, _, _ string, _ time.Duration, _ error)     {}
func (NoopReporter) OnVertexSkipped(_ context.Context, _, _, _ string, _ SkipReason)                {}
func (NoopReporter) OnVertexFields(_ context.Context, _, _ string, _ FieldPhase, _ map[string]any)  {}
