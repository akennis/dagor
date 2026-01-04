package runtime

import (
	"sync"
	"sync/atomic"

	"github.com/wwz16/dagor/graph"
	"github.com/wwz16/dagor/operator"
)

type GraphState int32

const (
	GraphStateInit GraphState = iota
	GraphStateRunning
	GraphStateFinished
)

type GraphStatus struct {
	// graph state. accessed atomically.
	state int32

	// done chan. used to wait graph execution finished.
	done     chan struct{}
	doneOnce sync.Once

	// wait group. used to wait all goroutines execution finished.
	wg sync.WaitGroup

	// graph execution error.
	err atomic.Value

	// operator instance map. vertex name -> operator instance.
	// written during init, read concurrently during execution.
	ops map[string]operator.IOperator

	// field value map. vertex field name -> operator field value.
	// written during init, read concurrently during execution.
	fieldValues map[string]*FieldValue

	// vertex in degrees map. vertex -> in degrees.
	// written during init, accessed via atomic operations during execution.
	inDegrees map[*graph.Vertex]*int32

	// start vertices. these vertices can be executed immediately.
	// written during init, read once at start of Run.
	startVertices []*graph.Vertex

	// pending count. the number of vertices that are not executed.
	pendingCount atomic.Int32

	// vertex errors map. vertex -> error.
	vertexErrors sync.Map
}

func NewGraphStatus() *GraphStatus {
	return &GraphStatus{
		ops:          make(map[string]operator.IOperator),
		fieldValues:  make(map[string]*FieldValue),
		inDegrees:    make(map[*graph.Vertex]*int32),
		vertexErrors: sync.Map{},
		done:         make(chan struct{}),
		state:        int32(GraphStateInit),
	}
}

// State returns the current graph state.
func (s *GraphStatus) State() GraphState {
	return GraphState(atomic.LoadInt32(&s.state))
}

// SetState sets the graph state atomically.
func (s *GraphStatus) SetState(state GraphState) {
	atomic.StoreInt32(&s.state, int32(state))
}

func (s *GraphStatus) SetError(err error) {
	if err == nil {
		return
	}
	// Use CompareAndSwap to ensure only first error is stored.
	if s.err.Load() != nil {
		return
	}

	// store error.
	s.err.Store(err)

	// notify done chan to notify graph execution finished.
	s.NotifyDone()
}

func (s *GraphStatus) Error() error {
	err := s.err.Load()
	if err == nil {
		return nil
	}
	return err.(error)
}

// VertexError returns the error for a vertex. Thread-safe.
func (s *GraphStatus) VertexError(v *graph.Vertex) error {
	err, ok := s.vertexErrors.Load(v)
	if !ok {
		return nil
	}
	return err.(error)
}

// SetVertexError sets the error for a vertex. Thread-safe.
func (s *GraphStatus) SetVertexError(v *graph.Vertex, err error) {
	if err == nil {
		return
	}
	s.vertexErrors.Store(v, err)
}

// Ops returns the operators. Thread-safe.
func (s *GraphStatus) Ops() map[string]operator.IOperator {
	return s.ops
}

// Op returns the operator for a vertex. Thread-safe.
func (s *GraphStatus) Op(vertexName string) (operator.IOperator, bool) {
	op, ok := s.ops[vertexName]
	return op, ok
}

// SetOp sets the operator for a vertex. Called during init only.
func (s *GraphStatus) SetOp(vertexName string, op operator.IOperator) {
	if op == nil {
		return
	}
	s.ops[vertexName] = op
}

// FieldValue returns the field value by name. Thread-safe.
func (s *GraphStatus) FieldValue(fieldName string) (*FieldValue, bool) {
	field, ok := s.fieldValues[fieldName]
	return field, ok
}

// SetFieldValue sets the field value. Called during init only.
func (s *GraphStatus) SetFieldValue(fieldName string, field *FieldValue) {
	s.fieldValues[fieldName] = field
}

// SetInDegree sets the in-degree for a vertex. Called during init only.
func (s *GraphStatus) SetInDegree(v *graph.Vertex, inDegree int32) {
	s.inDegrees[v] = &inDegree
}

// AddInDegree adds the in-degree for a vertex. Called during execution only.
func (s *GraphStatus) AddInDegree(v *graph.Vertex, n int32) int32 {
	inDegree, ok := s.inDegrees[v]
	if !ok {
		return 0
	}
	return atomic.AddInt32(inDegree, n)
}

// InDegree returns a pointer to the in-degree for a vertex.
// The returned pointer is safe for atomic operations.
func (s *GraphStatus) InDegree(v *graph.Vertex) int32 {
	inDegree, ok := s.inDegrees[v]
	if !ok {
		return 0
	}
	return atomic.LoadInt32(inDegree)
}

// AddStartVertex adds a start vertex. Called during init only.
func (s *GraphStatus) AddStartVertex(v *graph.Vertex) {
	s.startVertices = append(s.startVertices, v)
}

// StartVertices returns the start vertices. Safe to call after init.
func (s *GraphStatus) StartVertices() []*graph.Vertex {
	return s.startVertices
}

// NotifyDone notifies the graph execution finished.
func (s *GraphStatus) NotifyDone() {
	s.doneOnce.Do(func() {
		close(s.done)
	})
}

// Done returns the done chan.
func (s *GraphStatus) Done() <-chan struct{} {
	return s.done
}

// PendingCount returns the pending count.
func (s *GraphStatus) PendingCount() int32 {
	return s.pendingCount.Load()
}

// DecreasePendingCount decreases the pending count.
func (s *GraphStatus) DecreasePendingCount() int32 {
	return s.pendingCount.Add(-1)
}

// SetPendingCount sets the pending count.
func (s *GraphStatus) SetPendingCount(n int32) {
	s.pendingCount.Store(n)
}

// Wait waits for all goroutines execution finished.
func (s *GraphStatus) Wait() {
	s.wg.Wait()
}

// AddWaitGroup adds a wait group.
func (s *GraphStatus) AddWaitGroup(n int) {
	s.wg.Add(n)
}

// DoneWaitGroup decrements the wait group.
func (s *GraphStatus) DoneWaitGroup() {
	s.wg.Done()
}
