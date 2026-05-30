package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/akennis/dagor/examples/math/op"

	"github.com/panjf2000/ants/v2"
	"github.com/akennis/dagor"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/reporter"
)

func main() {
	// 1. Init global goroutine pool.
	// Take ants as an example, you may change to other pools.
	p, err := ants.NewPool(3)
	if err != nil {
		log.Printf("ants.NewPool error %v\n", err)
		return
	}
	defer p.Release()

	// 2. Build graph.
	// Read graph config file.
	content, err := os.ReadFile("./conf/math_demo.json")
	if err != nil {
		log.Printf("read file error %v\n", err)
		return
	}

	// Build graph.
	g, err := graph.NewGraphFromJson(content)
	if err != nil {
		log.Printf("NewGraphFromJson error %v\n", err)
		return
	}

	// 3. Run engine.
	// Operands are injected via context; the same compiled graph can be reused
	// across calls with different values by passing a fresh context each time.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, op.MathAKey, 10)
	ctx = context.WithValue(ctx, op.MathBKey, 20)

	// Set up structured logging at DEBUG level so all vertex events are visible.
	slogLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// FieldScrubber: demonstrate how to redact sensitive fields.
	// Fields named "secret" are omitted; all others pass through unchanged.
	scrubber := dagor.FieldScrubber(func(_ context.Context,
		_, _, fieldName string, _ dagor.FieldPhase, value any,
	) any {
		if fieldName == "secret" {
			return nil
		}
		return value
	})

	// Create engine instance.
	eng, err := dagor.NewEngine(g, p,
		dagor.WithReporter(reporter.New(slogLogger)),
		dagor.WithFieldScrubber(scrubber),
	)
	if err != nil {
		log.Printf("NewEngine error %v\n", err)
		return
	}
	defer eng.Close(ctx)

	// Run engine.
	if err = eng.Run(ctx); err != nil {
		log.Printf("Run error %v\n", err)
		return
	}

	// 4. Get the output data.
	v, ok := eng.GetOutput("answer")
	if !ok {
		log.Printf("GetOutput error\n")
		return
	}
	res := *v.(*float64)
	log.Printf("result: %f\n", res)
}
