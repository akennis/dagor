package graph

import (
	"encoding/json"
	"fmt"

	"github.com/wwz16/dagor/config"
)

// Graph is a graph of vertices and edges.
type Graph struct {
	name string

	// vertex name -> vertex
	vertices map[string]*Vertex

	// vertex field name -> vertex that output this field
	fieldVertex map[string]*Vertex
}

func NewGraphFromJson(jsonRaw json.RawMessage) (*Graph, error) {
	var config config.GraphConfig
	if err := json.Unmarshal(jsonRaw, &config); err != nil {
		return nil, err
	}
	return NewGraphFromConfig(&config)
}

// NewGraphFromConfig builds graph from config.
// It returns error if the config is invalid or the graph has cycle.
func NewGraphFromConfig(config *config.GraphConfig) (*Graph, error) {
	if config == nil {
		return nil, fmt.Errorf("graph config is required")
	}

	graph := &Graph{
		name:        config.Name,
		vertices:    make(map[string]*Vertex),
		fieldVertex: make(map[string]*Vertex),
	}

	if err := graph.initFromConfig(config); err != nil {
		return nil, err
	}
	return graph, nil
}

func (g *Graph) initFromConfig(config *config.GraphConfig) error {
	// create vertices
	for name, vconfig := range config.Vertices {
		if name == "" {
			return fmt.Errorf("vertex name is required")
		}
		if vconfig == nil {
			return fmt.Errorf("vertex config is required")
		}
		// check if vertex already exists
		if _, ok := g.vertices[name]; ok {
			return fmt.Errorf("vertex %s already exists", name)
		}

		// create vertex
		vertex, err := NewVertex(name, vconfig)
		if err != nil {
			return err
		}
		g.vertices[name] = vertex

		// update field vertex map
		for _, vertexField := range vertex.Outputs {
			if _, ok := g.fieldVertex[vertexField]; ok {
				return fmt.Errorf("field %s already exists", vertexField)
			}
			g.fieldVertex[vertexField] = vertex
		}
	}

	// create edges
	for name, vertex := range g.vertices {
		for _, input := range vertex.Inputs {
			predecessor, ok := g.fieldVertex[input]
			if !ok {
				return fmt.Errorf("predecessor vertex %s not found", input)
			}

			// update vertex edges
			vertex.predecessors[predecessor.name] = predecessor
			predecessor.successors[name] = vertex
		}
	}

	// check if there is any cycle
	if g.hasCycle() {
		return fmt.Errorf("graph has cycle")
	}
	return nil
}

// hasCycle checks if the graph has cycle.
// It uses the topological sort algorithm to check if the graph has cycle.
func (g *Graph) hasCycle() bool {
	// calculate in degree of each vertex
	inDegree := make(map[string]int)
	for _, vertex := range g.vertices {
		inDegree[vertex.name] = len(vertex.predecessors)
	}

	// find all vertices with in degree 0
	queue := make([]*Vertex, 0)
	for _, vertex := range g.vertices {
		if inDegree[vertex.name] == 0 {
			queue = append(queue, vertex)
		}
	}

	// topological sort
	visitCount := 0
	for len(queue) > 0 {
		vertex := queue[0]
		queue = queue[1:]

		for _, successor := range vertex.successors {
			inDegree[successor.name]--
			if inDegree[successor.name] == 0 {
				queue = append(queue, successor)
			}
		}
		visitCount++
	}

	// if visit count is not equal to the number of vertices, there is a cycle
	if visitCount != len(g.vertices) {
		return true
	}
	return false
}

// Name returns the name of the graph.
func (g *Graph) Name() string {
	return g.name
}

// Vertices returns the vertices of the graph.
func (g *Graph) Vertices() map[string]*Vertex {
	return g.vertices
}

// VertexByName returns the vertex by name.
func (g *Graph) VertexByName(name string) *Vertex {
	return g.vertices[name]
}

// Size returns the size of the graph.
func (g *Graph) Size() int {
	return len(g.vertices)
}
