package dagor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"

	"github.com/wwz16/dagor/config"
	"github.com/wwz16/dagor/graph"
	"github.com/wwz16/dagor/operator"
	"github.com/wwz16/dagor/predicate"
	"github.com/wwz16/dagor/reducer"
	"github.com/wwz16/dagor/runtime"
)

// opPooler abstracts operator pool get/put so tests can inject a tracking
// implementation without touching the operator package.
type opPooler interface {
	getOp(name string) (operator.IOperator, error)
	putOp(name string, op operator.IOperator) error
}

type defaultOpPool struct{}

func (defaultOpPool) getOp(name string) (operator.IOperator, error)  { return operator.GetOp(name) }
func (defaultOpPool) putOp(name string, op operator.IOperator) error { return operator.PutOp(name, op) }

// Engine is the engine for running the graph.
type Engine struct {
	graph  *graph.Graph
	pool   runtime.IGPool
	opPool opPooler
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
		opPool: defaultOpPool{},
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
		if v.Map != nil {
			// Map vertex: no operator. Pre-create a placeholder FieldValue for
			// the results wire; runMapVertex will fill it in during execution.
			if v.Map.ResultsWire != "" {
				e.status.SetFieldValue(v.Map.ResultsWire, &runtime.FieldValue{Name: v.Map.ResultsWire})
			}
			inDegree := int32(len(v.Predecessors()))
			e.status.SetInDegree(v, inDegree)
			if inDegree == 0 {
				e.status.AddStartVertex(v)
			}
			continue
		}
		if v.Filter != nil {
			// Filter vertex: no operator. Pre-create a placeholder FieldValue for
			// the results wire; runFilterVertex will fill it in during execution.
			if v.Filter.ResultsWire != "" {
				e.status.SetFieldValue(v.Filter.ResultsWire, &runtime.FieldValue{Name: v.Filter.ResultsWire})
			}
			inDegree := int32(len(v.Predecessors()))
			e.status.SetInDegree(v, inDegree)
			if inDegree == 0 {
				e.status.AddStartVertex(v)
			}
			continue
		}
		if v.Reduce != nil {
			// Reduce vertex: no operator. Pre-create a placeholder FieldValue for
			// the results wire; runReduceVertex will fill it in during execution.
			if v.Reduce.ResultsWire != "" {
				e.status.SetFieldValue(v.Reduce.ResultsWire, &runtime.FieldValue{Name: v.Reduce.ResultsWire})
			}
			inDegree := int32(len(v.Predecessors()))
			e.status.SetInDegree(v, inDegree)
			if inDegree == 0 {
				e.status.AddStartVertex(v)
			}
			continue
		}

		// init operator instance
		op, err := e.opPool.getOp(v.Op)
		if err != nil {
			e.releaseOps()
			return fmt.Errorf("get operator %s node %s error: %v", v.Op, v.Name(), err)
		}
		// setup operator.
		if err := op.Setup(v.Params()); err != nil {
			// Return the just-fetched op before releasing the rest; it has not
			// been stored in e.status.ops yet so releaseOps() won't find it.
			_ = e.opPool.putOp(v.Op, op)
			e.releaseOps()
			return fmt.Errorf("setup operator %s node %s error: %v", v.Op, v.Name(), err)
		}
		e.status.SetOp(v.Name(), op)

		// output fields binding.
		// Cache OutputFields() call to avoid repeated map lookups
		outputFields := op.OutputFields()
		for opFieldName, vertexFieldName := range v.Outputs {
			opField, ok := outputFields[opFieldName]
			if !ok {
				e.releaseOps()
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

// releaseOps returns all operators that were stored in e.status.ops back to
// their respective pools. Called on init failure to prevent pool leaks.
func (e *Engine) releaseOps() {
	for _, v := range e.graph.Vertices() {
		if v.Map != nil || v.Filter != nil || v.Reduce != nil {
			continue
		}
		if op, ok := e.status.Op(v.Name()); ok {
			_ = e.opPool.putOp(v.Op, op)
		}
	}
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

	// check if this vertex should be skipped.
	skip, err := e.shouldSkip(v)
	if err != nil {
		e.status.SetVertexError(v, err)
		if v.OnError == config.OnErrorStop {
			e.status.SetError(err)
			return err
		}
		skip = true // on_error=continue: skip to avoid propagating bad data
	}
	if skip {
		e.status.SetVertexSkipped(v)
		for opFieldName, vertexFieldName := range v.Outputs {
			fv, ok := e.status.FieldValue(vertexFieldName)
			if !ok {
				continue
			}
			if sourceWire, hasPT := v.PassthroughWires[opFieldName]; hasPT {
				if sourceFV, ok := e.status.FieldValue(sourceWire); ok {
					fv.Value = sourceFV.Value
				} else {
					fv.Value = nil
				}
			} else {
				fv.Value = nil
			}
		}
		return e.scheduleNextVertices(ctx, v)
	}

	// execute vertex (map, filter, reduce, or regular operator).
	var execErr error
	if v.Map != nil {
		execErr = e.runMapVertex(ctx, v)
	} else if v.Filter != nil {
		execErr = e.runFilterVertex(ctx, v)
	} else if v.Reduce != nil {
		execErr = e.runReduceVertex(ctx, v)
	} else {
		execErr = e.runOp(ctx, v)
	}
	if execErr != nil {
		e.status.SetVertexError(v, execErr)
		if v.OnError == config.OnErrorStop {
			e.status.SetError(execErr)
			return execErr
		}
		// OnErrorContinue: mirror the skip path so successors see consistent nil
		// outputs and propagate the skip transitively, rather than reading stale
		// or partially-written values from a failed operator.
		e.status.SetVertexSkipped(v)
		for opFieldName, vertexFieldName := range v.Outputs {
			fv, ok := e.status.FieldValue(vertexFieldName)
			if !ok {
				continue
			}
			if sourceWire, hasPT := v.PassthroughWires[opFieldName]; hasPT {
				if sourceFV, ok := e.status.FieldValue(sourceWire); ok {
					fv.Value = sourceFV.Value
				} else {
					fv.Value = nil
				}
			} else {
				fv.Value = nil
			}
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

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("run operator %s panic: %v", v.Name(), r)
		}
	}()

	// inject input fields dynamically.
	// performance optimization: only inject when needed.
	for opFieldName, vertexFieldName := range v.Inputs {
		field, ok := e.status.FieldValue(vertexFieldName)
		// A missing field (!ok) covers sub-graph edge cases.
		// A nil Value covers the common flat-graph case where the producer was
		// skipped and its FieldValue.Value was cleared in runVertex.
		if !ok || field.Value == nil {
			if v.Merge == config.MergeCoalesce {
				continue
			}
			err = fmt.Errorf("input field %s has nil value (producer may have been skipped)", vertexFieldName)
			return
		}
		if operr := op.SetInputField(opFieldName, field.Value); operr != nil {
			err = fmt.Errorf("set input field %s error: %v", opFieldName, operr)
			return
		}
	}

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

	var errs []error
	// reset operators.
	for _, v := range e.graph.Vertices() {
		if v.Map != nil || v.Filter != nil || v.Reduce != nil {
			continue // map/filter/reduce vertices have no operator to reset
		}
		op, ok := e.status.Op(v.Name())
		if !ok {
			continue
		}
		// reset input and output fields.
		op.ResetFields()
		// reset user defined fields; on failure drop the op so the pool
		// gets a fresh instance next time instead of a potentially dirty one.
		if err := op.Reset(); err != nil {
			log.Printf("reset operator %s error: %v\n", v.Name(), err)
			errs = append(errs, fmt.Errorf("reset %s: %w", v.Name(), err))
			continue
		}
		// put operator back to pool.
		if err := e.opPool.putOp(v.Op, op); err != nil {
			log.Printf("put operator %s error: %v\n", v.Op, err)
		}
	}
	return errors.Join(errs...)
}

// shouldSkip returns true if the vertex should be skipped.
// A vertex is skipped if any of its input-producing vertices was skipped,
// or if its own condition predicate evaluates to false.
// Exception: vertices with Merge == MergeCoalesce are only skipped when ALL
// of their input-producing vertices were skipped.
func (e *Engine) shouldSkip(v *graph.Vertex) (skip bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			skip = false
			err = fmt.Errorf("condition predicate %q panic: %v", v.Condition, r)
		}
	}()

	if v.Merge == config.MergeCoalesce {
		// Coalesce: skip only when every input-producing vertex was skipped.
		hasProducer := false
		for _, vertexFieldName := range v.Inputs {
			producer, ok := e.graph.FieldProducer(vertexFieldName)
			if !ok {
				continue
			}
			hasProducer = true
			if !e.status.IsVertexSkipped(producer) {
				// At least one producer ran — do not skip.
				goto evalCondition
			}
		}
		if hasProducer {
			return true, nil // every producer was skipped
		}
	} else {
		// Default: skip if any input-producing vertex was skipped.
		for _, vertexFieldName := range v.Inputs {
			producer, ok := e.graph.FieldProducer(vertexFieldName)
			if ok && e.status.IsVertexSkipped(producer) {
				return true, nil
			}
		}
	}

	// For reduce vertices, also skip when the InitWire producer was skipped.
	if v.Reduce != nil && v.Reduce.InitWire != "" {
		if producer, ok := e.graph.FieldProducer(v.Reduce.InitWire); ok && e.status.IsVertexSkipped(producer) {
			return true, nil
		}
	}

evalCondition:

	// Evaluate own condition predicate.
	if v.Condition == "" {
		return false, nil
	}
	pred, err := predicate.Get(v.Condition)
	if err != nil {
		return false, err
	}
	inputs := make(map[string]any, len(v.Inputs)+len(v.ConditionInputs))
	for _, vertexFieldName := range v.Inputs {
		field, ok := e.status.FieldValue(vertexFieldName)
		if !ok {
			return false, fmt.Errorf("condition input field %s not found", vertexFieldName)
		}
		inputs[vertexFieldName] = field.Value
	}
	for _, wireName := range v.ConditionInputs {
		if producer, ok := e.graph.FieldProducer(wireName); ok && e.status.IsVertexSkipped(producer) {
			return true, nil
		}
		field, ok := e.status.FieldValue(wireName)
		if !ok {
			return false, fmt.Errorf("condition input wire %s not found", wireName)
		}
		inputs[wireName] = field.Value
	}
	return !pred(inputs), nil
}

// runMapVertex executes a map vertex by fanning out sub-graph execution over
// every element of the input slice, then collecting results into a []any output.
func (e *Engine) runMapVertex(ctx context.Context, v *graph.Vertex) error {
	// Resolve the single input wire (the slice).
	// NewVertex enforces exactly one entry in v.Inputs, so this loop
	// always executes once. Go map iteration is not deterministic in general,
	// but the single-entry invariant makes order irrelevant here.
	var inputWire string
	for _, wire := range v.Inputs {
		inputWire = wire
	}
	fv, ok := e.status.FieldValue(inputWire)
	if !ok || fv.Value == nil {
		return fmt.Errorf("map vertex %s: input wire %q is nil or missing", v.Name(), inputWire)
	}

	// Use reflection to dereference a pointer and iterate the slice.
	rv := reflect.ValueOf(fv.Value)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return fmt.Errorf("map vertex %s: input wire %q is not a slice (got %T)", v.Name(), inputWire, fv.Value)
	}

	n := rv.Len()
	results := make([]any, n)

	// Sub-graphs run sequentially in the current goroutine to avoid deadlock:
	// if element goroutines were submitted to e.pool, they would occupy all
	// pool slots while waiting for their own sub-engine vertices to be
	// scheduled on the same pool — causing a deadlock with any bounded pool.
	for i := range n {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		elem := rv.Index(i)

		// Wrap the element in a pointer so sub-graph operators receive *T,
		// consistent with dagor's pointer-based wire convention.
		elemPtr := reflect.New(elem.Type())
		elemPtr.Elem().Set(elem)
		item := elemPtr.Interface()

		result, err := e.runSubgraph(ctx, v.Map, item)
		if err != nil {
			return fmt.Errorf("map vertex %s: element %d: %w", v.Name(), i, err)
		}
		results[i] = result
	}

	// Write collected results to the output wire declared on MapConfig.
	// Store a pointer so downstream operators receive *[]any, consistent with
	// dagor's pointer-based wire convention.
	if outFV, ok := e.status.FieldValue(v.Map.ResultsWire); ok {
		outFV.Value = &results
	}
	return nil
}

// runFilterVertex applies v.Filter.Predicate to each element of the input slice,
// collecting elements for which the predicate returns true into a []any output.
// Elements are wrapped in a *T pointer before being passed to the predicate,
// consistent with dagor's pointer-based wire convention. The kept elements are
// stored as their concrete (dereferenced) values in the result slice.
func (e *Engine) runFilterVertex(ctx context.Context, v *graph.Vertex) error {
	// Resolve the single input wire (the slice).
	// NewVertex enforces exactly one entry in v.Inputs.
	var inputWire string
	for _, wire := range v.Inputs {
		inputWire = wire
	}
	fv, ok := e.status.FieldValue(inputWire)
	if !ok || fv.Value == nil {
		return fmt.Errorf("filter vertex %s: input wire %q is nil or missing", v.Name(), inputWire)
	}

	rv := reflect.ValueOf(fv.Value)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return fmt.Errorf("filter vertex %s: input wire %q is not a slice (got %T)", v.Name(), inputWire, fv.Value)
	}

	pred, err := predicate.Get(v.Filter.Predicate)
	if err != nil {
		return fmt.Errorf("filter vertex %s: %w", v.Name(), err)
	}

	itemKey := v.Filter.ItemKey
	if itemKey == "" {
		itemKey = "item"
	}

	n := rv.Len()
	results := make([]any, 0, n)

	for i := range n {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		elem := rv.Index(i)

		// Wrap in a pointer so the predicate receives *T, consistent with
		// dagor's pointer-based wire convention and the MapOver convention.
		elemPtr := reflect.New(elem.Type())
		elemPtr.Elem().Set(elem)

		inputs := map[string]any{itemKey: elemPtr.Interface()}
		if pred(inputs) {
			results = append(results, elem.Interface())
		}
	}

	// Write the filtered slice to the output wire as *[]any, consistent with
	// dagor's pointer-based wire convention.
	if outFV, ok := e.status.FieldValue(v.Filter.ResultsWire); ok {
		outFV.Value = &results
	}
	return nil
}

// runReduceVertex folds the input slice into a single accumulated value using the
// registered reducer function. When InitWire is set the wire's value is used as
// the initial accumulator; otherwise the first element acts as the seed and
// reduction starts from the second element. An empty slice with no InitWire
// produces a nil result. The final value is written directly to ResultsWire
// (no extra pointer wrapping), so downstream operators receive the concrete type.
func (e *Engine) runReduceVertex(ctx context.Context, v *graph.Vertex) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("reduce vertex %s: reducer panicked: %v", v.Name(), r)
		}
	}()

	// Resolve the single input wire (the slice).
	// NewVertex enforces exactly one entry in v.Inputs.
	var inputWire string
	for _, wire := range v.Inputs {
		inputWire = wire
	}
	fv, ok := e.status.FieldValue(inputWire)
	if !ok || fv.Value == nil {
		return fmt.Errorf("reduce vertex %s: input wire %q is nil or missing", v.Name(), inputWire)
	}

	rv := reflect.ValueOf(fv.Value)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return fmt.Errorf("reduce vertex %s: input wire %q is not a slice (got %T)", v.Name(), inputWire, fv.Value)
	}

	fn, err := reducer.Get(v.Reduce.Reducer)
	if err != nil {
		return fmt.Errorf("reduce vertex %s: %w", v.Name(), err)
	}

	n := rv.Len()

	var acc any
	startIdx := 0

	if v.Reduce.InitWire != "" {
		initFV, ok := e.status.FieldValue(v.Reduce.InitWire)
		if !ok || initFV.Value == nil {
			return fmt.Errorf("reduce vertex %s: init wire %q is nil or missing", v.Name(), v.Reduce.InitWire)
		}
		acc = initFV.Value
	} else if n > 0 {
		acc = rv.Index(0).Interface()
		startIdx = 1
	}
	// Empty slice + no InitWire → acc remains nil.

	for i := startIdx; i < n; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		acc = fn(acc, rv.Index(i).Interface())
	}

	if outFV, ok := e.status.FieldValue(v.Reduce.ResultsWire); ok {
		outFV.Value = acc
	}
	return nil
}

// runSubgraph builds and executes the map sub-graph for a single item,
// returning the value of the designated result wire.
func (e *Engine) runSubgraph(ctx context.Context, mapCfg *config.MapConfig, item any) (any, error) {
	subGraph, err := graph.NewGraphFromConfig(mapCfg.Subgraph)
	if err != nil {
		return nil, fmt.Errorf("build subgraph: %w", err)
	}

	subEngine, err := NewEngine(subGraph, e.pool)
	if err != nil {
		return nil, err
	}

	defer subEngine.Close(ctx) //nolint:errcheck

	// Inject the item wire before Run so sub-graph start vertices can read it.
	subEngine.status.SetFieldValue(mapCfg.ItemInput, &runtime.FieldValue{
		Name:  mapCfg.ItemInput,
		Value: item,
	})

	if err := subEngine.Run(ctx); err != nil {
		return nil, err
	}

	result, ok := subEngine.GetOutput(mapCfg.ResultOutput)
	if !ok || result == nil {
		return nil, fmt.Errorf("result wire %q not found in subgraph output", mapCfg.ResultOutput)
	}

	// Dereference pointer so callers receive the concrete value, not *T.
	if rv := reflect.ValueOf(result); rv.Kind() == reflect.Ptr && !rv.IsNil() {
		result = rv.Elem().Interface()
	}
	return result, nil
}

// VertexSkipped reports whether the named vertex was skipped during the last Run.
func (e *Engine) VertexSkipped(name string) bool {
	v := e.graph.VertexByName(name)
	if v == nil {
		return false
	}
	return e.status.IsVertexSkipped(v)
}

// GetOutput gets the output by field name.
func (e *Engine) GetOutput(field string) (any, bool) {
	fieldValue, ok := e.status.FieldValue(field)
	if !ok {
		return nil, false
	}
	return fieldValue.Value, true
}
