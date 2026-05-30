package main

import (
	"context"
	"log"
	"time"

	_ "github.com/akennis/dagor/examples/temperature/op"

	"github.com/panjf2000/ants/v2"
	"github.com/akennis/dagor"
	"github.com/akennis/dagor/graph"
)

// buildTemperatureGraph builds:
//
//	source ──► [map: per-element pipeline]
//	               item (°C *float64)
//	                 → to_fahrenheit : Celsius → Fahrenheit (°F)
//	                 → categorize    : Fahrenheit → Category (string)
//	           collect "category" → "categories"
func buildTemperatureGraph(celsiusValues []float64) (*graph.Graph, error) {
	return graph.NewBuilder("temperature_pipeline").
		Vertex("source").
		Op("TempSourceOp").
		Params(map[string]any{"temps": celsiusValues}).
		Output("Temps", "celsius_temps").
		Vertex("classify_all").
		Input("Items", "celsius_temps").
		MapOver("item").
		SubVertex("to_fahrenheit").
		Op("ToFahrenheitOp").
		Input("Celsius", "item").
		Output("Fahrenheit", "fahrenheit").
		SubVertex("categorize").
		Op("CategorizeOp").
		Input("Temp", "fahrenheit").
		Output("Category", "category").
		CollectInto("category", "categories").
		Build()
}

func main() {
	p, err := ants.NewPool(10)
	if err != nil {
		log.Fatalf("ants.NewPool error: %v", err)
	}
	defer p.Release()

	input := []float64{-10, 0, 20, 37, 100}

	g, err := buildTemperatureGraph(input)
	if err != nil {
		log.Fatalf("buildTemperatureGraph error: %v", err)
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

	out, ok := eng.GetOutput("categories")
	if !ok {
		log.Fatal("categories wire not found")
	}

	results := *out.(*[]any)
	log.Printf("input (°C) → category")
	for i, cat := range results {
		log.Printf("  %.0f°C → %s", input[i], cat.(string))
	}
}
