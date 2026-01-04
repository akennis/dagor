package graph

import (
	"encoding/json"
	"testing"

	"github.com/wwz16/dagor/config"
)

func TestNewGraphFromConfig_NilConfig(t *testing.T) {
	graph, err := NewGraphFromConfig(nil)
	if err == nil {
		t.Error("expected error for nil config, got nil")
	}
	if graph != nil {
		t.Error("expected nil graph for nil config")
	}
}

func TestNewGraphFromConfig_EmptyConfig(t *testing.T) {
	cfg := &config.GraphConfig{
		Name:     "test",
		Vertices: make(map[string]*config.VertexConfig),
	}

	graph, err := NewGraphFromConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if graph == nil {
		t.Error("expected non-nil graph")
	}
	if graph.Name() != "test" {
		t.Errorf("expected graph name 'test', got '%s'", graph.Name())
	}
}

func TestNewGraphFromConfig_EmptyVertexName(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"": {
				Op:     "test_op",
				Params: json.RawMessage(`{}`),
				Inputs: make(map[string]string),
				Outputs: map[string]string{
					"out": "field1",
				},
			},
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err == nil {
		t.Error("expected error for empty vertex name, got nil")
	}
	if graph != nil {
		t.Error("expected nil graph for invalid config")
	}
}

func TestNewGraphFromConfig_NilVertexConfig(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"v1": nil,
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err == nil {
		t.Error("expected error for nil vertex config, got nil")
	}
	if graph != nil {
		t.Error("expected nil graph for invalid config")
	}
}

func TestNewGraphFromConfig_DuplicateVertexName(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test",
		Vertices: map[string]*config.VertexConfig{
			"v1": {
				Op:     "test_op",
				Params: json.RawMessage(`{}`),
				Inputs: make(map[string]string),
				Outputs: map[string]string{
					"out": "field1",
				},
			},
		},
	}

	// This shouldn't happen in practice, but we test the duplicate check
	// by creating a graph and trying to add the same vertex twice
	// Actually, the map structure prevents this, so we test a different scenario
	// Let's test with duplicate field names instead
	cfg.Vertices["v2"] = &config.VertexConfig{
		Op:     "test_op",
		Params: json.RawMessage(`{}`),
		Inputs: make(map[string]string),
		Outputs: map[string]string{
			"out": "field1", // duplicate field name
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err == nil {
		t.Error("expected error for duplicate field name, got nil")
	}
	if graph != nil {
		t.Error("expected nil graph for invalid config")
	}
}

func TestNewGraphFromConfig_ValidSimpleGraph(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test_graph",
		Vertices: map[string]*config.VertexConfig{
			"v1": {
				Op:     "op1",
				Params: json.RawMessage(`{"key": "value"}`),
				Inputs: make(map[string]string),
				Outputs: map[string]string{
					"out": "field1",
				},
			},
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	if graph.Name() != "test_graph" {
		t.Errorf("expected graph name 'test_graph', got '%s'", graph.Name())
	}
	if len(graph.Vertices()) != 1 {
		t.Errorf("expected 1 vertex, got %d", len(graph.Vertices()))
	}
	if graph.VertexByName("v1") == nil {
		t.Error("expected vertex v1 to exist")
	}
	if len(graph.fieldVertex) != 1 {
		t.Errorf("expected 1 field vertex mapping, got %d", len(graph.fieldVertex))
	}
	if graph.fieldVertex["field1"] != graph.VertexByName("v1") {
		t.Error("expected field1 to map to v1")
	}
}

func TestNewGraphFromConfig_ValidGraphWithEdges(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test_graph",
		Vertices: map[string]*config.VertexConfig{
			"v1": {
				Op:     "op1",
				Params: json.RawMessage(`{}`),
				Inputs: make(map[string]string),
				Outputs: map[string]string{
					"out": "field1",
				},
			},
			"v2": {
				Op:     "op2",
				Params: json.RawMessage(`{}`),
				Inputs: map[string]string{
					"in": "field1",
				},
				Outputs: map[string]string{
					"out": "field2",
				},
			},
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	if len(graph.Vertices()) != 2 {
		t.Errorf("expected 2 vertices, got %d", len(graph.Vertices()))
	}

	v1 := graph.VertexByName("v1")
	v2 := graph.VertexByName("v2")

	// Check v1 has v2 as successor
	if len(v1.Successors()) != 1 {
		t.Errorf("expected v1 to have 1 successor, got %d", len(v1.Successors()))
	}
	if v1.Successors()["v2"] != v2 {
		t.Error("expected v1 to have v2 as successor")
	}

	// Check v2 has v1 as predecessor
	if len(v2.Predecessors()) != 1 {
		t.Errorf("expected v2 to have 1 predecessor, got %d", len(v2.Predecessors()))
	}
	if v2.Predecessors()["v1"] != v1 {
		t.Error("expected v2 to have v1 as predecessor")
	}
}

func TestNewGraphFromConfig_MissingPredecessor(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test_graph",
		Vertices: map[string]*config.VertexConfig{
			"v1": {
				Op:     "op1",
				Params: json.RawMessage(`{}`),
				Inputs: map[string]string{
					"in": "nonexistent_field",
				},
				Outputs: map[string]string{
					"out": "field1",
				},
			},
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err == nil {
		t.Error("expected error for missing predecessor, got nil")
	}
	if graph != nil {
		t.Error("expected nil graph for invalid config")
	}
}

func TestNewGraphFromConfig_CycleDetection(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test_graph",
		Vertices: map[string]*config.VertexConfig{
			"v1": {
				Op:     "op1",
				Params: json.RawMessage(`{}`),
				Inputs: map[string]string{
					"in": "field2",
				},
				Outputs: map[string]string{
					"out": "field1",
				},
			},
			"v2": {
				Op:     "op2",
				Params: json.RawMessage(`{}`),
				Inputs: map[string]string{
					"in": "field1",
				},
				Outputs: map[string]string{
					"out": "field2",
				},
			},
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err == nil {
		t.Error("expected error for cycle, got nil")
	}
	if err != nil && err.Error() != "graph has cycle" {
		t.Errorf("expected 'graph has cycle' error, got: %v", err)
	}
	if graph != nil {
		t.Error("expected nil graph for invalid config")
	}
}

func TestNewGraphFromConfig_ComplexGraph(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "complex_graph",
		Vertices: map[string]*config.VertexConfig{
			"v1": {
				Op:     "op1",
				Params: json.RawMessage(`{"a": 1}`),
				Inputs: make(map[string]string),
				Outputs: map[string]string{
					"out1": "field1",
					"out2": "field2",
				},
			},
			"v2": {
				Op:     "op2",
				Params: json.RawMessage(`{"b": 2}`),
				Inputs: map[string]string{
					"in1": "field1",
				},
				Outputs: map[string]string{
					"out": "field3",
				},
			},
			"v3": {
				Op:     "op3",
				Params: json.RawMessage(`{"c": 3}`),
				Inputs: map[string]string{
					"in1": "field2",
					"in2": "field3",
				},
				Outputs: map[string]string{
					"out": "field4",
				},
			},
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}
	if len(graph.Vertices()) != 3 {
		t.Errorf("expected 3 vertices, got %d", len(graph.Vertices()))
	}
	if len(graph.fieldVertex) != 4 {
		t.Errorf("expected 4 field vertex mappings, got %d", len(graph.fieldVertex))
	}

	// Check v1 has 2 successors
	v1 := graph.VertexByName("v1")
	if len(v1.Successors()) != 2 {
		t.Errorf("expected v1 to have 2 successors, got %d", len(v1.Successors()))
	}

	// Check v2 has 1 predecessor and 1 successor
	v2 := graph.VertexByName("v2")
	if len(v2.Predecessors()) != 1 {
		t.Errorf("expected v2 to have 1 predecessor, got %d", len(v2.Predecessors()))
	}
	if len(v2.Successors()) != 1 {
		t.Errorf("expected v2 to have 1 successor, got %d", len(v2.Successors()))
	}

	// Check v3 has 2 predecessors
	v3 := graph.VertexByName("v3")
	if len(v3.Predecessors()) != 2 {
		t.Errorf("expected v3 to have 2 predecessors, got %d", len(v3.Predecessors()))
	}
}

func TestHasCycle_NoCycle(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test_graph",
		Vertices: map[string]*config.VertexConfig{
			"v1": {
				Op:     "op1",
				Params: json.RawMessage(`{}`),
				Inputs: make(map[string]string),
				Outputs: map[string]string{
					"out": "field1",
				},
			},
			"v2": {
				Op:     "op2",
				Params: json.RawMessage(`{}`),
				Inputs: map[string]string{
					"in": "field1",
				},
				Outputs: map[string]string{
					"out": "field2",
				},
			},
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating graph: %v", err)
	}

	if graph.hasCycle() {
		t.Error("expected no cycle, but cycle was detected")
	}
}

func TestHasCycle_WithCycle(t *testing.T) {
	// Create a graph with a cycle manually to test hasCycle directly
	graph := &Graph{
		name:        "test",
		vertices:    make(map[string]*Vertex),
		fieldVertex: make(map[string]*Vertex),
	}

	v1 := &Vertex{
		name:         "v1",
		successors:   make(map[string]*Vertex),
		predecessors: make(map[string]*Vertex),
	}
	v2 := &Vertex{
		name:         "v2",
		successors:   make(map[string]*Vertex),
		predecessors: make(map[string]*Vertex),
	}

	v1.successors["v2"] = v2
	v2.predecessors["v1"] = v1
	v2.successors["v1"] = v1
	v1.predecessors["v2"] = v2

	graph.vertices["v1"] = v1
	graph.vertices["v2"] = v2

	if !graph.hasCycle() {
		t.Error("expected cycle, but no cycle was detected")
	}
}

func TestHasCycle_SingleVertex(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test_graph",
		Vertices: map[string]*config.VertexConfig{
			"v1": {
				Op:     "op1",
				Params: json.RawMessage(`{}`),
				Inputs: make(map[string]string),
				Outputs: map[string]string{
					"out": "field1",
				},
			},
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating graph: %v", err)
	}

	if graph.hasCycle() {
		t.Error("expected no cycle for single vertex, but cycle was detected")
	}
}

func TestHasCycle_DisconnectedVertices(t *testing.T) {
	cfg := &config.GraphConfig{
		Name: "test_graph",
		Vertices: map[string]*config.VertexConfig{
			"v1": {
				Op:     "op1",
				Params: json.RawMessage(`{}`),
				Inputs: make(map[string]string),
				Outputs: map[string]string{
					"out": "field1",
				},
			},
			"v2": {
				Op:     "op2",
				Params: json.RawMessage(`{}`),
				Inputs: make(map[string]string),
				Outputs: map[string]string{
					"out": "field2",
				},
			},
		},
	}

	graph, err := NewGraphFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating graph: %v", err)
	}

	if graph.hasCycle() {
		t.Error("expected no cycle for disconnected vertices, but cycle was detected")
	}
}
