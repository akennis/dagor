package main

import (
	"context"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/wwz16/dagor"
	"github.com/wwz16/dagor/predicate"
)

// withPredicates registers the demo predicates for the duration of the test
// and unregisters them in t.Cleanup so successive runs (-count=2) stay clean.
func withPredicates(t *testing.T) {
	t.Helper()
	registerPredicates()
	t.Cleanup(func() {
		predicate.Unregister("positive")
		predicate.Unregister("negative")
	})
}

func TestRegisterPredicates_Idempotent(t *testing.T) {
	// Calling registerPredicates twice must not panic or crash.
	registerPredicates()
	registerPredicates()
	t.Cleanup(func() {
		predicate.Unregister("positive")
		predicate.Unregister("negative")
	})
}

func TestCoalesceDemo_Positive(t *testing.T) {
	withPredicates(t)

	p, err := ants.NewPool(3)
	if err != nil {
		t.Fatalf("ants.NewPool: %v", err)
	}
	defer p.Release()

	g, err := buildCoalesceGraph(5)
	if err != nil {
		t.Fatalf("buildCoalesceGraph(5): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eng, err := dagor.NewEngine(g, p)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close(ctx)

	if err = eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	v, ok := eng.GetOutput("final_out")
	if !ok {
		t.Fatal("GetOutput(final_out): not found")
	}
	got := *v.(*int)
	if got != 50 {
		t.Errorf("source=5 → want 50, got %d", got)
	}
}

func TestCoalesceDemo_Negative(t *testing.T) {
	withPredicates(t)

	p, err := ants.NewPool(3)
	if err != nil {
		t.Fatalf("ants.NewPool: %v", err)
	}
	defer p.Release()

	g, err := buildCoalesceGraph(-3)
	if err != nil {
		t.Fatalf("buildCoalesceGraph(-3): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eng, err := dagor.NewEngine(g, p)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close(ctx)

	if err = eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	v, ok := eng.GetOutput("final_out")
	if !ok {
		t.Fatal("GetOutput(final_out): not found")
	}
	got := *v.(*int)
	if got != 4 {
		t.Errorf("source=-3 → want 4, got %d", got)
	}
}
