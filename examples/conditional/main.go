package main

import (
	"context"
	"log"
	"time"

	_ "github.com/akennis/dagor/examples/conditional/op"
	_ "github.com/akennis/dagor/operator/builtin" // CoalesceIntOp

	"github.com/panjf2000/ants/v2"
	"github.com/akennis/dagor"
	"github.com/akennis/dagor/config"
	"github.com/akennis/dagor/graph"
	"github.com/akennis/dagor/predicate"
)

// registerPredicates installs the "positive" and "negative" predicates used by
// the coalesce demo. It uses MustReplace so it is safe to call more than once
// (e.g., from test setup without crashing on duplicate registration).
func registerPredicates() {
	// positive: execute the branch only when source output is positive.
	predicate.MustReplace("positive", func(inputs map[string]any) bool {
		ptr, ok := inputs["source_out"].(*int)
		if !ok || ptr == nil {
			return false
		}
		return *ptr > 0
	})
	// negativePredicate is defined explicitly (not as !positivePredicate) so that
	// zero routes to neither branch: both predicates return false, both branches
	// are skipped, and CoalesceIntOp receives nil for A and B and will error.
	// This makes the three cases explicit: >0 → positive, <0 → negative, 0 → error.
	predicate.MustReplace("negative", func(inputs map[string]any) bool {
		ptr, ok := inputs["source_out"].(*int)
		if !ok || ptr == nil {
			return false
		}
		return *ptr < 0
	})
}

// buildCoalesceGraph builds a graph where two mutually-exclusive branches feed
// a single CoalesceIntOp output node:
//
//	source ──► positive_branch (condition=positive)                            ──► coalesce (merge=coalesce)
//	       └─► negative_branch  (condition=negative) ──► negative_branch_step2 ──►
//
// PositiveBranchOp multiplies by 10; NegativeBranchOp negates (making negative→positive), NegativeBranchStep2Op adds 1.
// Note: NegativeBranchStep2 does not have a condition on it directly but will still only execute when the original input was negative
// due to the transitive nature of conditionals through sub-trees.
// Edge case: source == 0 causes both predicates to return false, both branches are skipped,
// and CoalesceIntOp receives nil for both inputs, resulting in a Run error.
func buildCoalesceGraph(sourceValue int) (*graph.Graph, error) {
	return graph.NewBuilder("coalesce_demo").
		Vertex("source").
		Op("SourceOp").
		Params(map[string]int{"value": sourceValue}).
		Output("out", "source_out").
		Done().
		Vertex("positive_branch").
		Op("PositiveBranchOp").
		Condition("positive").
		Input("in", "source_out").
		Output("out", "positive_out").
		Done().
		Vertex("negative_branch").
		Op("NegativeBranchOp").
		Condition("negative").
		Input("in", "source_out").
		Output("out", "negative_out").
		Done().
		Vertex("negative_branch_step2").
		Op("NegativeBranchStep2Op").
		Input("in", "negative_out").
		Output("out", "negative_step2_out").
		Done().
		Vertex("coalesce").
		Op("CoalesceIntOp").
		Merge(config.MergeCoalesce).
		Input("A", "positive_out").
		Input("B", "negative_step2_out").
		Output("Result", "final_out").
		Done().
		Build()
}

func runCoalesceDemo(pool *ants.Pool, sourceValue int) {
	g, err := buildCoalesceGraph(sourceValue)
	if err != nil {
		log.Printf("buildCoalesceGraph error: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	eng, err := dagor.NewEngine(g, pool)
	if err != nil {
		log.Printf("NewEngine error: %v\n", err)
		return
	}
	defer eng.Close(ctx)

	if err = eng.Run(ctx); err != nil {
		log.Printf("Run error: %v\n", err)
		return
	}

	v, _ := eng.GetOutput("final_out")
	log.Printf("[coalesce] source=%d → coalesced_out=%d\n",
		sourceValue, *v.(*int))
}

func main() {
	registerPredicates()

	p, err := ants.NewPool(3)
	if err != nil {
		log.Fatalf("ants.NewPool error: %v\n", err)
	}
	defer p.Release()

	log.Println("")
	log.Println("=== Two-branch coalesce demo ===")
	log.Println("--- Run with positive value (5): positive_branch runs : expecting result (50) ---")
	runCoalesceDemo(p, 5)
	log.Println("--- Run with negative value (-3): negative_branch runs : expecting result (4) ---")
	runCoalesceDemo(p, -3)
}
