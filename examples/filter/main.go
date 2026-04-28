package main

import (
	"context"
	"log"
	"time"

	_ "github.com/wwz16/dagor/examples/filter/op"

	"github.com/panjf2000/ants/v2"
	"github.com/wwz16/dagor"
	"github.com/wwz16/dagor/graph"
	"github.com/wwz16/dagor/predicate"
)

// registerPredicates installs the predicates used by the filter demos.
// Uses MustReplace so it is safe to call more than once (e.g. in test setup).
func registerPredicates() {
	// passing: keep scores of 60 or above.
	predicate.MustReplace("passing", func(inputs map[string]any) bool {
		ptr, ok := inputs["item"].(*int)
		return ok && ptr != nil && *ptr >= 60
	})
	// high_score: keep only scores of 90 or above.
	predicate.MustReplace("high_score", func(inputs map[string]any) bool {
		ptr, ok := inputs["item"].(*int)
		return ok && ptr != nil && *ptr >= 90
	})
}

// buildFilterGraph builds:
//
//	source ──► [filter: predicateName] ──► filtered_wire
//
// The filter vertex applies the named predicate to each element of the input
// slice and retains only matching elements in a []any output wire.
func buildFilterGraph(scores []int, predicateName string) (*graph.Graph, error) {
	return graph.NewBuilder("filter_demo").
		Vertex("source").
		Op("SourceOp").
		Params(map[string]any{"scores": scores}).
		Output("Scores", "scores_wire").
		Vertex("filter").
		Input("In", "scores_wire").
		FilterBy(predicateName).
		CollectInto("filtered_wire").
		Build()
}

// toInts converts a []any (each element an int) to []int for display.
func toInts(results []any) []int {
	out := make([]int, len(results))
	for i, v := range results {
		out[i] = v.(int)
	}
	return out
}

func runFilterDemo(pool *ants.Pool, scores []int, predicateName, label string) {
	g, err := buildFilterGraph(scores, predicateName)
	if err != nil {
		log.Printf("[%s] buildFilterGraph error: %v", label, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	eng, err := dagor.NewEngine(g, pool)
	if err != nil {
		log.Printf("[%s] NewEngine error: %v", label, err)
		return
	}
	defer eng.Close(ctx) //nolint:errcheck

	if err = eng.Run(ctx); err != nil {
		log.Printf("[%s] Run error: %v", label, err)
		return
	}

	out, ok := eng.GetOutput("filtered_wire")
	if !ok {
		log.Printf("[%s] filtered_wire not found", label)
		return
	}
	results := toInts(*out.(*[]any))
	log.Printf("[%s] input:    %v", label, scores)
	log.Printf("[%s] filtered: %v", label, results)
}

func main() {
	registerPredicates()

	p, err := ants.NewPool(4)
	if err != nil {
		log.Fatalf("ants.NewPool error: %v", err)
	}
	defer p.Release()

	scores := []int{45, 72, 55, 88, 91, 43, 76, 60, 95, 33}

	log.Println("")
	log.Println("=== Filter demo: exam scores ===")
	log.Println("--- Passing scores (>= 60) ---")
	runFilterDemo(p, scores, "passing", "passing")
	log.Println("--- High scores (>= 90) ---")
	runFilterDemo(p, scores, "high_score", "high_score")
}
