package graph

import (
	"fmt"

	"github.com/wwz16/dagor/config"
)

// Vertex is a vertex in the graph.
type Vertex struct {
	// vertex config
	config.VertexConfig

	// vertex data
	name         string
	params       *config.Params
	successors   map[string]*Vertex
	predecessors map[string]*Vertex
}

// NewVertex creates a new vertex.
func NewVertex(name string, vconfig *config.VertexConfig) (*Vertex, error) {
	if vconfig == nil {
		return nil, fmt.Errorf("vertex config is required")
	}
	if name == "" {
		return nil, fmt.Errorf("vertex name is required")
	}

	setCount := 0
	if vconfig.Op != "" {
		setCount++
	}
	if vconfig.Map != nil {
		setCount++
	}
	if vconfig.Filter != nil {
		setCount++
	}
	if vconfig.Reduce != nil {
		setCount++
	}
	if setCount > 1 {
		return nil, fmt.Errorf("vertex %q: Op, Map, Filter, and Reduce are mutually exclusive", name)
	}
	if setCount == 0 {
		return nil, fmt.Errorf("vertex %q: one of Op, Map, Filter, or Reduce must be set", name)
	}

	// normalize OnError locally — do not mutate the caller's config.
	onError := vconfig.OnError
	if onError != config.OnErrorStop && onError != config.OnErrorContinue {
		onError = config.OnErrorStop
	}

	if vconfig.Map != nil {
		if len(vconfig.Inputs) != 1 {
			return nil, fmt.Errorf("vertex %q: map vertex must have exactly one input wire, got %d", name, len(vconfig.Inputs))
		}
		if vconfig.Map.ResultsWire == "" {
			return nil, fmt.Errorf("vertex %q: map vertex must have a non-empty ResultsWire", name)
		}
	}

	if vconfig.Filter != nil {
		if len(vconfig.Inputs) != 1 {
			return nil, fmt.Errorf("vertex %q: filter vertex must have exactly one input wire, got %d", name, len(vconfig.Inputs))
		}
		if vconfig.Filter.Predicate == "" {
			return nil, fmt.Errorf("vertex %q: filter vertex must have a non-empty Predicate", name)
		}
		if vconfig.Filter.ResultsWire == "" {
			return nil, fmt.Errorf("vertex %q: filter vertex must have a non-empty ResultsWire", name)
		}
	}

	if vconfig.Reduce != nil {
		if len(vconfig.Inputs) != 1 {
			return nil, fmt.Errorf("vertex %q: reduce vertex must have exactly one input wire, got %d", name, len(vconfig.Inputs))
		}
		if vconfig.Reduce.Reducer == "" {
			return nil, fmt.Errorf("vertex %q: reduce vertex must have a non-empty Reducer", name)
		}
		if vconfig.Reduce.ResultsWire == "" {
			return nil, fmt.Errorf("vertex %q: reduce vertex must have a non-empty ResultsWire", name)
		}
	}

	params, err := config.NewFromRaw(vconfig.Params)
	if err != nil {
		return nil, err
	}
	v := &Vertex{
		VertexConfig: *vconfig,
		name:         name,
		params:       params,
		successors:   make(map[string]*Vertex),
		predecessors: make(map[string]*Vertex),
	}
	v.OnError = onError
	return v, nil
}

// Name returns the name of the vertex.
func (v *Vertex) Name() string {
	return v.name
}

// Params returns the params of the vertex.
func (v *Vertex) Params() *config.Params {
	return v.params
}

// Successors returns the successors of the vertex.
func (v *Vertex) Successors() map[string]*Vertex {
	return v.successors
}

// Predecessors returns the predecessors of the vertex.
func (v *Vertex) Predecessors() map[string]*Vertex {
	return v.predecessors
}
