package main

import (
	"context"
	"log"
	"os"
	"time"

	_ "github.com/wwz16/dagor/examples/math/op"
	_ "github.com/wwz16/dagor/operator/builtin"

	"github.com/panjf2000/ants/v2"
	"github.com/wwz16/dagor"
	"github.com/wwz16/dagor/graph"
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

	// 3. Run engine
	// Init context.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Create engine instance
	eng, err := dagor.NewEngine(g, p)
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
