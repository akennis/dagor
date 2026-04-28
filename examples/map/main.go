package main

import (
	"context"
	"log"
	"time"

	_ "github.com/wwz16/dagor/examples/map/op"

	"github.com/panjf2000/ants/v2"
	"github.com/wwz16/dagor"
	"github.com/wwz16/dagor/graph"
)

// buildMapGraph builds:
//
//	source ──► [map: double each element] ──► ([]any of doubled ints)
//
// The map node fans out a DoubleOp sub-graph over every element of the input
// slice concurrently, then assembles the results into a []any output wire.
func buildMapGraph() (*graph.Graph, error) {
	return graph.NewBuilder("map_demo").
		Vertex("source").
		Op("SourceOp").
		Params(map[string]any{"values": []int{1, 2, 3, 4, 5}}).
		Output("Items", "raw_items").
		Vertex("double_all").
		Input("Items", "raw_items").
		MapOver("item").
		SubVertex("double").
		Op("DoubleOp").
		Input("In", "item").
		Output("Out", "result").
		CollectInto("result", "doubled_items").
		Build()
}

func main() {
	p, err := ants.NewPool(10)
	if err != nil {
		log.Fatalf("ants.NewPool error: %v", err)
	}
	defer p.Release()

	g, err := buildMapGraph()
	if err != nil {
		log.Fatalf("buildMapGraph error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eng, err := dagor.NewEngine(g, p)
	if err != nil {
		log.Fatalf("NewEngine error: %v", err)
	}
	defer eng.Close(ctx)

	if err := eng.Run(ctx); err != nil {
		log.Fatalf("Run error: %v", err)
	}

	out, ok := eng.GetOutput("doubled_items")
	if !ok {
		log.Fatal("doubled_items wire not found")
	}

	results := *out.(*[]any)
	log.Printf("input:  [1 2 3 4 5]")
	log.Printf("output: %v", results)
}
