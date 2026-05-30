// Package reporter provides a built-in [dagor.Reporter] implementation backed by [log/slog].
package reporter

import (
	"context"
	"log/slog"
	"time"

	"github.com/akennis/dagor"
)

// SlogReporter implements [dagor.Reporter] by emitting structured log events via [*slog.Logger].
// It is goroutine-safe because [*slog.Logger] is safe for concurrent use.
// Every log line includes a run_id attribute populated from the context by [dagor.RunID].
type SlogReporter struct {
	logger *slog.Logger
}

// New returns a SlogReporter that writes to logger.
func New(logger *slog.Logger) *SlogReporter {
	return &SlogReporter{logger: logger}
}

// runIDAttr returns a slog.Attr for the run ID stored in ctx, or a zero Attr if none is set.
func runIDAttr(ctx context.Context) slog.Attr {
	id := dagor.RunID(ctx)
	if id == "" {
		return slog.Attr{}
	}
	return slog.String("run_id", id)
}

func (r *SlogReporter) OnGraphStart(ctx context.Context, name string) {
	r.logger.InfoContext(ctx, "graph.start",
		runIDAttr(ctx),
		slog.String("graph", name),
	)
}

func (r *SlogReporter) OnGraphFinish(ctx context.Context, name string, dur time.Duration, err error) {
	attrs := []any{
		runIDAttr(ctx),
		slog.String("graph", name),
		slog.Int64("dur_ms", dur.Milliseconds()),
	}
	if err != nil {
		r.logger.ErrorContext(ctx, "graph.finish", append(attrs, slog.Any("err", err))...)
	} else {
		r.logger.InfoContext(ctx, "graph.finish", attrs...)
	}
}

func (r *SlogReporter) OnVertexStart(ctx context.Context, graphName, vertexName, vertexType string) {
	r.logger.DebugContext(ctx, "vertex.start",
		runIDAttr(ctx),
		slog.String("graph", graphName),
		slog.String("vertex", vertexName),
		slog.String("type", vertexType),
	)
}

func (r *SlogReporter) OnVertexFinish(ctx context.Context, graphName, vertexName, vertexType string, dur time.Duration, err error) {
	attrs := []any{
		runIDAttr(ctx),
		slog.String("graph", graphName),
		slog.String("vertex", vertexName),
		slog.String("type", vertexType),
		slog.Int64("dur_ms", dur.Milliseconds()),
	}
	if err != nil {
		r.logger.ErrorContext(ctx, "vertex.finish", append(attrs, slog.Any("err", err))...)
	} else {
		r.logger.DebugContext(ctx, "vertex.finish", attrs...)
	}
}

func (r *SlogReporter) OnVertexSkipped(ctx context.Context, graphName, vertexName, vertexType string, reason dagor.SkipReason) {
	r.logger.DebugContext(ctx, "vertex.skipped",
		runIDAttr(ctx),
		slog.String("graph", graphName),
		slog.String("vertex", vertexName),
		slog.String("type", vertexType),
		slog.String("reason", string(reason)),
	)
}

func (r *SlogReporter) OnVertexFields(ctx context.Context, graphName, vertexName string, phase dagor.FieldPhase, fields map[string]any) {
	phaseStr := "input"
	if phase == dagor.FieldPhaseOutput {
		phaseStr = "output"
	}
	attrs := []any{
		runIDAttr(ctx),
		slog.String("graph", graphName),
		slog.String("vertex", vertexName),
		slog.String("phase", phaseStr),
	}
	for k, v := range fields {
		attrs = append(attrs, slog.Any(k, v))
	}
	r.logger.DebugContext(ctx, "vertex.fields", attrs...)
}
