package dagor

import (
	"context"
	"fmt"
	"log"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/graph"
	"github.com/wwz16/dagor/operator"
	"github.com/wwz16/dagor/runtime"
)

// Engine is the engine for running the graph.
type Engine struct {
	graph  *graph.Graph
	pool   runtime.IGPool
	status *runtime.GraphStatus
}

// NewEngine creates a new engine.
func NewEngine(graph *graph.Graph, pool runtime.IGPool) (*Engine, error) {
	// check params
	if graph == nil {
		return nil, fmt.Errorf("graph is required")
	}
	if pool == nil {
		return nil, fmt.Errorf("pool is required")
	}

	return &Engine{
		graph:  graph,
		pool:   pool,
		status: runtime.NewGraphStatus(),
	}, nil
}

// Run runs the graph.
func (e *Engine) Run(ctx context.Context) error {
	// check if graph is empty.
	if e.graph.Size() == 0 {
		return nil
	}

	// init engine.
	// setup operators and bind output fields.
	if err := e.init(); err != nil {
		return err
	}

	// start graph execution.
	e.status.SetState(runtime.GraphStateRunning)
	startVertices := e.status.StartVertices()
	for _, v := range startVertices {
		// submit op execution task to pool.
		e.status.AddWaitGroup(1)
		err := e.pool.Submit(func() {
			defer e.status.DoneWaitGroup()
			e.runVertex(ctx, v)
		})
		if err != nil {
			e.status.DoneWaitGroup()
			e.status.SetError(err)
			return err
		}
	}

	// wait graph execution finished.
	select {
	case <-e.status.Done():
	case <-ctx.Done():
		e.status.SetError(ctx.Err())
	}

	// wait all active goroutines execution finished.
	e.status.Wait()

	// set graph state to finished.
	e.status.SetState(runtime.GraphStateFinished)
	return e.status.Error()
}

// init initializes the engine.
func (e *Engine) init() error {
	e.status.SetPendingCount(int32(e.graph.Size()))

	for _, v := range e.graph.Vertices() {
		// init operator instance
		op, err := operator.GetOp(v.Op)
		if err != nil {
			return fmt.Errorf("get operator %s node %s error: %v", v.Op, v.Name(), err)
		}
		// setup operator.
		if err := op.Setup(v.Params()); err != nil {
			return fmt.Errorf("setup operator %s node %s error: %v", v.Op, v.Name(), err)
		}
		e.status.SetOp(v.Name(), op)

		// output fields binding.
		// Cache OutputFields() call to avoid repeated map lookups
		outputFields := op.OutputFields()
		for opFieldName, vertexFieldName := range v.Outputs {
			opField, ok := outputFields[opFieldName]
			if !ok {
				return fmt.Errorf("operator %s node %s output field %s not found", v.Op, v.Name(), opFieldName)
			}
			e.status.SetFieldValue(vertexFieldName, &runtime.FieldValue{
				Name:  opFieldName,
				Value: opField,
			})
		}

		// in degrees calculation.
		inDegree := int32(len(v.Predecessors()))
		e.status.SetInDegree(v, inDegree)
		if inDegree == 0 {
			e.status.AddStartVertex(v)
		}
	}

	return nil
}

// runVertex runs a vertex.
func (e *Engine) runVertex(ctx context.Context, v *graph.Vertex) error {
	// check if ctx is done (early return optimization).
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// check if all predecessors have been executed.
	if e.status.InDegree(v) != 0 {
		return fmt.Errorf("vertex %s in degree is not 0", v.Name())
	}

	// check if graph has execution error (early return optimization).
	if err := e.status.Error(); err != nil {
		return fmt.Errorf("graph has execution error: %v", err)
	}

	// execute operator.
	if err := e.runOp(ctx, v); err != nil {
		e.status.SetVertexError(v, err)
		if v.OnError == config.OnErrorStop {
			e.status.SetError(err)
			return err
		}
	}

	// schedule next vertices execution.
	return e.scheduleNextVertices(ctx, v)
}

// runOp executes the operator.
func (e *Engine) runOp(ctx context.Context, v *graph.Vertex) (err error) {
	// get operator.
	op, ok := e.status.Op(v.Name())
	if !ok {
		err = fmt.Errorf("operator %s not found", v.Name())
		return
	}

	// inject input fields dynamically.
	// performance optimization: only inject when needed.
	for opFieldName, vertexFieldName := range v.Inputs {
		field, ok := e.status.FieldValue(vertexFieldName)
		if !ok {
			err = fmt.Errorf("input field %s not found", vertexFieldName)
			return
		}
		if operr := op.SetInputField(opFieldName, field.Value); operr != nil {
			err = fmt.Errorf("set input field %s error: %v", opFieldName, operr)
			return
		}
	}

	defer func() {
		if r := recover(); r != nil {
			panicErr := fmt.Errorf("run operator %s panic: %v", v.Name(), r)
			err = panicErr
			return
		}
	}()

	// execute operator.
	return op.Run(ctx)
}

func (e *Engine) scheduleNextVertices(ctx context.Context, v *graph.Vertex) error {
	// check if ctx is done (early return optimization).
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// check graph done
	if e.graphIsDone() {
		e.status.NotifyDone()
		return nil
	}

	// find next vertex with in degree 0.
	// Pre-allocate with capacity hint for better performance.
	successorCount := len(v.Successors())
	readyVertices := make([]*graph.Vertex, 0, successorCount)
	for _, successor := range v.Successors() {
		if e.status.AddInDegree(successor, -1) == 0 {
			readyVertices = append(readyVertices, successor)
		}
	}
	if len(readyVertices) == 0 {
		return nil
	}

	// submit next vertex execution tasks to pool.
	// skip the first vertex, it will be executed in current goroutine.
	for _, successor := range readyVertices[1:] {
		e.status.AddWaitGroup(1)
		err := e.pool.Submit(func() {
			defer e.status.DoneWaitGroup()
			e.runVertex(ctx, successor)
		})
		if err != nil {
			e.status.DoneWaitGroup()
			e.status.SetError(err)
			return err
		}
	}

	// performance optimization: execute the first vertex in current goroutine to avoid context switching.
	return e.runVertex(ctx, readyVertices[0])
}

func (e *Engine) graphIsDone() bool {
	// decrease pending count and check if it is 0.
	if e.status.DecreasePendingCount() == 0 {
		return true
	}
	return false
}

// Close closes the engine.
// It should be called after Run.
func (e *Engine) Close(ctx context.Context) error {
	// check graph state
	if e.status.State() != runtime.GraphStateFinished {
		return fmt.Errorf("graph state is not finished, current state: %d", e.status.State())
	}

	// reset operators.
	for _, v := range e.graph.Vertices() {
		op, ok := e.status.Op(v.Name())
		if !ok {
			continue
		}
		// reset input and output fields.
		op.ResetFields()
		// reset user defined fields.
		if err := op.Reset(); err != nil {
			log.Printf("reset operator %s error: %v\n", v.Name(), err)
		}
		// put operator back to pool.
		if err := operator.PutOp(v.Op, op); err != nil {
			log.Printf("put operator %s error: %v\n", v.Op, err)
		}
	}
	return nil
}

// GetOutput gets the output by field name.
func (e *Engine) GetOutput(field string) (any, bool) {
	fieldValue, ok := e.status.FieldValue(field)
	if !ok {
		return nil, false
	}
	return fieldValue.Value, true
}
