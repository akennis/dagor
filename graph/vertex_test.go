package graph

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/akennis/dagor/config"
)

func TestNewVertex_NilConfig(t *testing.T) {
	vertex, err := NewVertex("test", nil)
	if err == nil {
		t.Error("expected error for nil config, got nil")
	}
	if vertex != nil {
		t.Error("expected nil vertex for nil config")
	}
}

func TestNewVertex_EmptyName(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}

	vertex, err := NewVertex("", vconfig)
	if err == nil {
		t.Error("expected error for empty name, got nil")
	}
	if vertex != nil {
		t.Error("expected nil vertex for empty name")
	}
}

func TestNewVertex_DefaultOnError(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
		OnError: "", // empty should default to "stop"
	}

	vertex, err := NewVertex("v1", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
	if vertex.Name() != "v1" {
		t.Errorf("expected vertex name 'v1', got '%s'", vertex.Name())
	}
	if vertex.OnError != config.OnErrorStop {
		t.Errorf("expected OnErrorStop, got %s", vertex.OnError)
	}
}

func TestNewVertex_InvalidOnError(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
		OnError: "invalid",
	}
	vertex, err := NewVertex("v1", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
	if vertex.OnError != config.OnErrorStop {
		t.Errorf("expected OnErrorStop, got %s", vertex.OnError)
	}
}

func TestNewVertex_Valid(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{"a":123}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
		OnError: config.OnErrorStop,
	}
	vertex, err := NewVertex("v1", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
	if vertex.Name() != "v1" {
		t.Errorf("expected vertex name 'v1', got '%s'", vertex.Name())
	}
	if vertex.OnError != config.OnErrorStop {
		t.Errorf("expected OnErrorStop, got %s", vertex.OnError)
	}
	if vertex.Params() == nil {
		t.Error("expected non-nil params")
	}
	if len(vertex.Successors()) != 0 {
		t.Errorf("expected 0 successors, got %d", len(vertex.Successors()))
	}
	if len(vertex.Predecessors()) != 0 {
		t.Errorf("expected 0 predecessors, got %d", len(vertex.Predecessors()))
	}
	if vertex.Op != "test_op" {
		t.Errorf("expected test_op, got %s", vertex.Op)
	}
	if vertex.Params().GetInt("a", 0) != 123 {
		t.Errorf("expected 123, got %d", vertex.Params().GetInt("a", 0))
	}
}

func TestNewVertex_OnErrorContinue(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
		OnError: config.OnErrorContinue,
	}

	vertex, err := NewVertex("v1", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
	if vertex.OnError != config.OnErrorContinue {
		t.Errorf("expected OnErrorContinue, got %s", vertex.OnError)
	}
}

func TestNewVertex_InvalidJSONParams(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{"invalid": json}`), // invalid JSON
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}

	vertex, err := NewVertex("v1", vconfig)
	if err == nil {
		t.Error("expected error for invalid JSON params, got nil")
	}
	if vertex != nil {
		t.Error("expected nil vertex for invalid JSON params")
	}
}

func TestNewVertex_EmptyParams(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}

	vertex, err := NewVertex("v1", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
	if vertex.Params() == nil {
		t.Error("expected non-nil params even for empty JSON")
	}
	// Should return default value for non-existent key
	if vertex.Params().GetInt("nonexistent", 42) != 42 {
		t.Errorf("expected default value 42, got %d", vertex.Params().GetInt("nonexistent", 42))
	}
}

func TestNewVertex_NilParams(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  nil,
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}

	vertex, err := NewVertex("v1", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
	if vertex.Params() == nil {
		t.Error("expected non-nil params even for nil JSON")
	}
}

func TestNewVertex_ComplexParams(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{"a":123,"b":"hello","c":true,"d":45.67,"e":[1,2,3],"f":{"nested":"value"}}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}

	vertex, err := NewVertex("v1", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}

	// Test various param types
	if vertex.Params().GetInt("a", 0) != 123 {
		t.Errorf("expected 123, got %d", vertex.Params().GetInt("a", 0))
	}
	if vertex.Params().GetString("b", "") != "hello" {
		t.Errorf("expected 'hello', got '%s'", vertex.Params().GetString("b", ""))
	}
	if vertex.Params().GetBool("c", false) != true {
		t.Errorf("expected true, got %v", vertex.Params().GetBool("c", false))
	}
	if vertex.Params().GetFloat64("d", 0) != 45.67 {
		t.Errorf("expected 45.67, got %f", vertex.Params().GetFloat64("d", 0))
	}

	// Test array
	arr := vertex.Params().GetArray("e")
	if arr == nil {
		t.Error("expected non-nil array")
	} else if len(arr) != 3 {
		t.Errorf("expected array length 3, got %d", len(arr))
	}

	// Test nested object
	subParams := vertex.Params().GetSubParams("f")
	if subParams == nil {
		t.Error("expected non-nil sub params")
	} else if subParams.GetString("nested", "") != "value" {
		t.Errorf("expected nested value 'value', got '%s'", subParams.GetString("nested", ""))
	}
}

func TestNewVertex_InputsOutputs(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:     "test_op",
		Params: json.RawMessage(`{}`),
		Inputs: map[string]string{
			"input1": "field1",
			"input2": "field2",
		},
		Outputs: map[string]string{
			"output1": "out_field1",
			"output2": "out_field2",
		},
	}

	vertex, err := NewVertex("v1", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}

	// Check that inputs are preserved
	if len(vertex.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(vertex.Inputs))
	}
	if vertex.Inputs["input1"] != "field1" {
		t.Errorf("expected input1->field1, got %s", vertex.Inputs["input1"])
	}
	if vertex.Inputs["input2"] != "field2" {
		t.Errorf("expected input2->field2, got %s", vertex.Inputs["input2"])
	}

	// Check that outputs are preserved
	if len(vertex.Outputs) != 2 {
		t.Errorf("expected 2 outputs, got %d", len(vertex.Outputs))
	}
	if vertex.Outputs["output1"] != "out_field1" {
		t.Errorf("expected output1->out_field1, got %s", vertex.Outputs["output1"])
	}
	if vertex.Outputs["output2"] != "out_field2" {
		t.Errorf("expected output2->out_field2, got %s", vertex.Outputs["output2"])
	}
}

func TestNewVertex_SuccessorsPredecessorsInitialized(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}

	vertex, err := NewVertex("v1", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}

	// Check that maps are initialized (not nil)
	if vertex.Successors() == nil {
		t.Error("expected non-nil successors map")
	}
	if vertex.Predecessors() == nil {
		t.Error("expected non-nil predecessors map")
	}

	// Check that maps are empty but writable
	if len(vertex.Successors()) != 0 {
		t.Errorf("expected 0 successors, got %d", len(vertex.Successors()))
	}
	if len(vertex.Predecessors()) != 0 {
		t.Errorf("expected 0 predecessors, got %d", len(vertex.Predecessors()))
	}
}

func TestVertex_Methods(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{"key":"value"}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}

	vertex, err := NewVertex("test_vertex", vconfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test Name() method
	if name := vertex.Name(); name != "test_vertex" {
		t.Errorf("expected name 'test_vertex', got '%s'", name)
	}

	// Test Params() method
	params := vertex.Params()
	if params == nil {
		t.Fatal("expected non-nil params")
	}
	if params.GetString("key", "") != "value" {
		t.Errorf("expected param value 'value', got '%s'", params.GetString("key", ""))
	}

	// Test Successors() method
	successors := vertex.Successors()
	if successors == nil {
		t.Error("expected non-nil successors")
	}
	if len(successors) != 0 {
		t.Errorf("expected 0 successors, got %d", len(successors))
	}

	// Test Predecessors() method
	predecessors := vertex.Predecessors()
	if predecessors == nil {
		t.Error("expected non-nil predecessors")
	}
	if len(predecessors) != 0 {
		t.Errorf("expected 0 predecessors, got %d", len(predecessors))
	}
}

func TestNewVertex_WhitespaceName(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}

	// Test with whitespace-only name (implementation only checks for empty string, not whitespace)
	vertex, err := NewVertex("   ", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
	if vertex.Name() != "   " {
		t.Errorf("expected name '   ', got '%s'", vertex.Name())
	}
}

func TestNewVertex_MapVertex_ValidSingleInputOutput(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs:  map[string]string{"slice_field": "slice_wire"},
		Outputs: map[string]string{},
		Map: &config.MapConfig{
			ItemInput:    "item_in",
			ResultOutput: "item_out",
			ResultsWire:  "results_wire",
			Subgraph:     &config.GraphConfig{},
		},
	}
	vertex, err := NewVertex("map_v", vconfig)
	if err != nil {
		t.Errorf("unexpected error for valid map vertex: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
}

func TestNewVertex_MapVertex_ZeroInputs(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs:  map[string]string{},
		Outputs: map[string]string{},
		Map: &config.MapConfig{
			ItemInput:    "item_in",
			ResultOutput: "item_out",
			ResultsWire:  "results_wire",
			Subgraph:     &config.GraphConfig{},
		},
	}
	_, err := NewVertex("map_v", vconfig)
	if err == nil {
		t.Error("expected error for map vertex with zero inputs, got nil")
	}
}

func TestNewVertex_MapVertex_TwoInputs(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs: map[string]string{
			"field_a": "wire_a",
			"field_b": "wire_b",
		},
		Outputs: map[string]string{},
		Map: &config.MapConfig{
			ItemInput:    "item_in",
			ResultOutput: "item_out",
			ResultsWire:  "results_wire",
			Subgraph:     &config.GraphConfig{},
		},
	}
	_, err := NewVertex("map_v", vconfig)
	if err == nil {
		t.Error("expected error for map vertex with two inputs, got nil")
	}
}

func TestNewVertex_MapVertex_EmptyResultsWire(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs:  map[string]string{"slice_field": "slice_wire"},
		Outputs: map[string]string{},
		Map: &config.MapConfig{
			ItemInput:    "item_in",
			ResultOutput: "item_out",
			ResultsWire:  "", // missing — must error
			Subgraph:     &config.GraphConfig{},
		},
	}
	_, err := NewVertex("map_v", vconfig)
	if err == nil {
		t.Error("expected error for map vertex with empty ResultsWire, got nil")
	}
}

func TestNewVertex_DoesNotMutateCallerConfig(t *testing.T) {
	// BUG-05: NewVertex must not write back to the caller's *VertexConfig.
	// If it did, concurrent calls sharing the same config would race.
	cases := []struct {
		name    string
		onError string
	}{
		{"empty defaults to stop without mutating", ""},
		{"invalid defaults to stop without mutating", "bogus"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.onError
			vconfig := &config.VertexConfig{
				Op:      "test_op",
				Params:  json.RawMessage(`{}`),
				Inputs:  make(map[string]string),
				Outputs: make(map[string]string),
				OnError: original,
			}
			vertex, err := NewVertex("v1", vconfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if vertex.OnError != config.OnErrorStop {
				t.Errorf("vertex.OnError: want %q, got %q", config.OnErrorStop, vertex.OnError)
			}
			if vconfig.OnError != original {
				t.Errorf("caller config mutated: want %q, got %q", original, vconfig.OnError)
			}
		})
	}
}

func TestNewVertex_BothOpAndMap_Error(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:     "some_op",
		Params: json.RawMessage(`{}`),
		Inputs: map[string]string{"slice_field": "slice_wire"},
		Map: &config.MapConfig{
			ItemInput:    "item_in",
			ResultOutput: "item_out",
			ResultsWire:  "results_wire",
			Subgraph:     &config.GraphConfig{},
		},
	}
	_, err := NewVertex("v", vconfig)
	if err == nil {
		t.Fatal("expected error when both Op and Map are set, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

func TestNewVertex_NeitherOpNorMap_Error(t *testing.T) {
	vconfig := &config.VertexConfig{
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}
	_, err := NewVertex("v", vconfig)
	if err == nil {
		t.Fatal("expected error when neither Op nor Map is set, got nil")
	}
	if !strings.Contains(err.Error(), "one of Op, Map, Filter, or Reduce must be set") {
		t.Errorf("expected 'one of Op, Map, Filter, or Reduce must be set' in error, got: %v", err)
	}
}

func TestNewVertex_FilterVertex_Valid(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs:  map[string]string{"slice_field": "slice_wire"},
		Outputs: map[string]string{},
		Filter: &config.FilterConfig{
			Predicate:   "positive",
			ResultsWire: "filtered_wire",
		},
	}
	vertex, err := NewVertex("filter_v", vconfig)
	if err != nil {
		t.Errorf("unexpected error for valid filter vertex: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
	if vertex.Filter == nil {
		t.Fatal("expected non-nil Filter on vertex")
	}
	if vertex.Filter.Predicate != "positive" {
		t.Errorf("expected Predicate 'positive', got %q", vertex.Filter.Predicate)
	}
	if vertex.Filter.ResultsWire != "filtered_wire" {
		t.Errorf("expected ResultsWire 'filtered_wire', got %q", vertex.Filter.ResultsWire)
	}
}

func TestNewVertex_FilterVertex_ZeroInputs(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs:  map[string]string{},
		Outputs: map[string]string{},
		Filter: &config.FilterConfig{
			Predicate:   "positive",
			ResultsWire: "filtered_wire",
		},
	}
	_, err := NewVertex("filter_v", vconfig)
	if err == nil {
		t.Error("expected error for filter vertex with zero inputs, got nil")
	}
}

func TestNewVertex_FilterVertex_TwoInputs(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs: map[string]string{
			"field_a": "wire_a",
			"field_b": "wire_b",
		},
		Outputs: map[string]string{},
		Filter: &config.FilterConfig{
			Predicate:   "positive",
			ResultsWire: "filtered_wire",
		},
	}
	_, err := NewVertex("filter_v", vconfig)
	if err == nil {
		t.Error("expected error for filter vertex with two inputs, got nil")
	}
}

func TestNewVertex_FilterVertex_EmptyPredicate(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs:  map[string]string{"slice_field": "slice_wire"},
		Outputs: map[string]string{},
		Filter: &config.FilterConfig{
			Predicate:   "",
			ResultsWire: "filtered_wire",
		},
	}
	_, err := NewVertex("filter_v", vconfig)
	if err == nil {
		t.Error("expected error for filter vertex with empty Predicate, got nil")
	}
}

func TestNewVertex_FilterVertex_EmptyResultsWire(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs:  map[string]string{"slice_field": "slice_wire"},
		Outputs: map[string]string{},
		Filter: &config.FilterConfig{
			Predicate:   "positive",
			ResultsWire: "",
		},
	}
	_, err := NewVertex("filter_v", vconfig)
	if err == nil {
		t.Error("expected error for filter vertex with empty ResultsWire, got nil")
	}
}

func TestNewVertex_FilterAndOp_MutuallyExclusive(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:     "some_op",
		Inputs: map[string]string{"slice_field": "slice_wire"},
		Filter: &config.FilterConfig{
			Predicate:   "positive",
			ResultsWire: "filtered_wire",
		},
	}
	_, err := NewVertex("filter_v", vconfig)
	if err == nil {
		t.Fatal("expected error when both Op and Filter are set, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

func TestNewVertex_FilterAndMap_MutuallyExclusive(t *testing.T) {
	vconfig := &config.VertexConfig{
		Inputs: map[string]string{"slice_field": "slice_wire"},
		Filter: &config.FilterConfig{
			Predicate:   "positive",
			ResultsWire: "filtered_wire",
		},
		Map: &config.MapConfig{
			ItemInput:    "item_in",
			ResultOutput: "item_out",
			ResultsWire:  "results_wire",
			Subgraph:     &config.GraphConfig{},
		},
	}
	_, err := NewVertex("v", vconfig)
	if err == nil {
		t.Fatal("expected error when both Filter and Map are set, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

func TestNewVertex_SpecialCharactersInName(t *testing.T) {
	vconfig := &config.VertexConfig{
		Op:      "test_op",
		Params:  json.RawMessage(`{}`),
		Inputs:  make(map[string]string),
		Outputs: make(map[string]string),
	}

	// Test with special characters (should be allowed)
	vertex, err := NewVertex("vertex-123_test", vconfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if vertex == nil {
		t.Fatal("expected non-nil vertex")
	}
	if vertex.Name() != "vertex-123_test" {
		t.Errorf("expected name 'vertex-123_test', got '%s'", vertex.Name())
	}
}
