package main

import (
	"context"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/akennis/dagor"
	"github.com/akennis/dagor/predicate"
)

// withPredicates registers the demo predicates for the duration of the test
// and unregisters them in t.Cleanup so successive runs (-count=2) stay clean.
func withPredicates(t *testing.T) {
	t.Helper()
	registerPredicates()
	t.Cleanup(func() {
		predicate.Unregister("passing")
		predicate.Unregister("high_score")
	})
}

func TestRegisterPredicates_Idempotent(t *testing.T) {
	registerPredicates()
	registerPredicates()
	t.Cleanup(func() {
		predicate.Unregister("passing")
		predicate.Unregister("high_score")
	})
}

func TestFilterDemo_Passing(t *testing.T) {
	withPredicates(t)

	p, err := ants.NewPool(4)
	if err != nil {
		t.Fatalf("ants.NewPool: %v", err)
	}
	defer p.Release()

	scores := []int{45, 72, 55, 88, 91, 43, 76, 60, 95, 33}
	g, err := buildFilterGraph(scores, "passing")
	if err != nil {
		t.Fatalf("buildFilterGraph: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eng, err := dagor.NewEngine(g, p)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close(ctx) //nolint:errcheck

	if err = eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		t.Fatal("filtered_wire not found")
	}
	got := toInts(*out.(*[]any))
	want := []int{72, 88, 91, 76, 60, 95}
	if len(got) != len(want) {
		t.Fatalf("passing filter: want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("result[%d]: want %d, got %d", i, w, got[i])
		}
	}
}

func TestFilterDemo_HighScore(t *testing.T) {
	withPredicates(t)

	p, err := ants.NewPool(4)
	if err != nil {
		t.Fatalf("ants.NewPool: %v", err)
	}
	defer p.Release()

	scores := []int{45, 72, 55, 88, 91, 43, 76, 60, 95, 33}
	g, err := buildFilterGraph(scores, "high_score")
	if err != nil {
		t.Fatalf("buildFilterGraph: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eng, err := dagor.NewEngine(g, p)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close(ctx) //nolint:errcheck

	if err = eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		t.Fatal("filtered_wire not found")
	}
	got := toInts(*out.(*[]any))
	want := []int{91, 95}
	if len(got) != len(want) {
		t.Fatalf("high_score filter: want %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("result[%d]: want %d, got %d", i, w, got[i])
		}
	}
}

func TestFilterDemo_AllFiltered(t *testing.T) {
	withPredicates(t)

	p, err := ants.NewPool(4)
	if err != nil {
		t.Fatalf("ants.NewPool: %v", err)
	}
	defer p.Release()

	scores := []int{10, 20, 30} // all below 60
	g, err := buildFilterGraph(scores, "passing")
	if err != nil {
		t.Fatalf("buildFilterGraph: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eng, err := dagor.NewEngine(g, p)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close(ctx) //nolint:errcheck

	if err = eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		t.Fatal("filtered_wire not found")
	}
	got := toInts(*out.(*[]any))
	if len(got) != 0 {
		t.Errorf("expected empty result when all filtered out, got %v", got)
	}
}
