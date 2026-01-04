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

	// set default on error action.
	if vconfig.OnError == "" {
		vconfig.OnError = config.OnErrorStop
	}
	if vconfig.OnError != config.OnErrorStop && vconfig.OnError != config.OnErrorContinue {
		vconfig.OnError = config.OnErrorStop
	}

	params, err := config.NewFromRaw(vconfig.Params)
	if err != nil {
		return nil, err
	}
	return &Vertex{
		VertexConfig: *vconfig,
		name:         name,
		params:       params,
		successors:   make(map[string]*Vertex),
		predecessors: make(map[string]*Vertex),
	}, nil
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
