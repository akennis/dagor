package graph

import (
	"strings"
	"testing"

	"github.com/wwz16/dagor/config"
)

func TestBuilder_MultiError(t *testing.T) {
	_, err := NewBuilder("test_graph").
		Vertex("v1").
		Op("").          // Error: empty op
		Input("in", ""). // Error: empty vertexField
		Vertex("v2").
		Params(make(chan int)). // Error: marshal failure
		Vertex("").             // Error: empty vertex name
		Build()

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	expectedErrors := []string{
		"graph \"test_graph\": vertex name cannot be empty",
		"vertex \"v1\": operator name cannot be empty",
		"vertex \"v1\": input mapping cannot have empty fields",
		"vertex \"v2\": failed to marshal params",
	}

	for _, expected := range expectedErrors {
		if !strings.Contains(errMsg, expected) {
			t.Errorf("expected error to contain %q, but it was %q", expected, errMsg)
		}
	}
}

func TestBuilder_FullFeatures(t *testing.T) {
	g, err := NewBuilder("full_graph").
		Vertex("source").
		Op("SourceOp").
		Output("out", "source_wire").
		Vertex("processor").
		Op("ProcessOp").
		Params(map[string]int{"threshold": 10}).
		Input("in", "source_wire").
		Condition("is_valid").
		ConditionInput("source_wire").
		PassthroughWire("result", "source_wire").
		Merge("first").
		OnError("continue").
		Output("result", "final_wire").
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v := g.VertexByName("processor")
	if v == nil {
		t.Fatal("vertex 'processor' not found")
	}

	// Verify all fields were correctly mapped
	if v.Op != "ProcessOp" {
		t.Errorf("expected Op 'ProcessOp', got %q", v.Op)
	}
	if v.Condition != "is_valid" {
		t.Errorf("expected Condition 'is_valid', got %q", v.Condition)
	}
	if len(v.ConditionInputs) != 1 || v.ConditionInputs[0] != "source_wire" {
		t.Errorf("unexpected ConditionInputs: %v", v.ConditionInputs)
	}
	if v.PassthroughWires["result"] != "source_wire" {
		t.Errorf("unexpected PassthroughWires: %v", v.PassthroughWires)
	}
	if v.Merge != "first" {
		t.Errorf("expected Merge 'first', got %q", v.Merge)
	}
	if v.OnError != "continue" {
		t.Errorf("expected OnError 'continue', got %q", v.OnError)
	}
}

func TestBuilder_VertexReentry(t *testing.T) {
	// Verify that calling Vertex() twice for the same name returns the same builder
	// and preserves state.
	b := NewBuilder("reentry_test")

	b.Vertex("source").Op("SourceOp").Output("out", "wire")
	vb1 := b.Vertex("v1").Op("Op1")
	vb2 := b.Vertex("v1").Input("in", "wire")

	if vb1 != vb2 {
		t.Fatal("Vertex() returned different builders for the same name")
	}

	g, err := b.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v := g.VertexByName("v1")
	if v.Op != "Op1" || v.Inputs["in"] != "wire" {
		t.Errorf("vertex state not preserved across re-entry: Op=%q, Inputs=%v", v.Op, v.Inputs)
	}
}

func TestBuilder_NestedMapOver(t *testing.T) {
	g, err := NewBuilder("nested_map").
		Vertex("source").
		Op("SourceOp").
		Output("Items", "outer_list").
		Vertex("outer_map").
		Input("Items", "outer_list").
		MapOver("outer_item").
		SubVertex("inner_map").
		Input("Items", "outer_item").
		MapOver("inner_item").
		SubVertex("double").
		Op("DoubleOp").
		Input("In", "inner_item").
		Output("Out", "inner_result").
		CollectInto("inner_result", "inner_results").
		CollectInto("inner_results", "outer_results").
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outerMap := g.VertexByName("outer_map")
	if outerMap == nil {
		t.Fatal("vertex 'outer_map' not found")
	}
	if outerMap.Map == nil {
		t.Fatal("outer_map: expected Map config")
	}
	if outerMap.Map.ItemInput != "outer_item" {
		t.Errorf("outer_map: ItemInput = %q, want %q", outerMap.Map.ItemInput, "outer_item")
	}
	if outerMap.Map.ResultOutput != "inner_results" {
		t.Errorf("outer_map: ResultOutput = %q, want %q", outerMap.Map.ResultOutput, "inner_results")
	}
	if outerMap.Map.ResultsWire != "outer_results" {
		t.Errorf("outer_map: ResultsWire = %q, want %q", outerMap.Map.ResultsWire, "outer_results")
	}
	if _, hasKey := outerMap.Outputs["Results"]; hasKey {
		t.Error("outer_map: Outputs must not contain the magic 'Results' key after BUG-02 fix")
	}

	innerMapCfg, ok := outerMap.Map.Subgraph.Vertices["inner_map"]
	if !ok {
		t.Fatal("outer sub-graph: vertex 'inner_map' not found")
	}
	if innerMapCfg.Map == nil {
		t.Fatal("inner_map: expected Map config")
	}
	if innerMapCfg.Map.ItemInput != "inner_item" {
		t.Errorf("inner_map: ItemInput = %q, want %q", innerMapCfg.Map.ItemInput, "inner_item")
	}
	if innerMapCfg.Map.ResultOutput != "inner_result" {
		t.Errorf("inner_map: ResultOutput = %q, want %q", innerMapCfg.Map.ResultOutput, "inner_result")
	}
	if innerMapCfg.Map.ResultsWire != "inner_results" {
		t.Errorf("inner_map: ResultsWire = %q, want %q", innerMapCfg.Map.ResultsWire, "inner_results")
	}
	if _, hasKey := innerMapCfg.Outputs["Results"]; hasKey {
		t.Error("inner_map: Outputs must not contain the magic 'Results' key after BUG-02 fix")
	}

	doubleCfg, ok := innerMapCfg.Map.Subgraph.Vertices["double"]
	if !ok {
		t.Fatal("inner sub-graph: vertex 'double' not found")
	}
	if doubleCfg.Op != "DoubleOp" {
		t.Errorf("double: Op = %q, want %q", doubleCfg.Op, "DoubleOp")
	}
}

// TestBuilder_MapCollectInto_DoubleCallError verifies that calling CollectInto
// twice on the same MapConfigBuilder produces an error (BUG-02 fix).
func TestBuilder_MapCollectInto_DoubleCallError(t *testing.T) {
	mcb := &MapConfigBuilder{
		vb: &VertexBuilder{
			config: &config.VertexConfig{
				Inputs:  make(map[string]string),
				Outputs: make(map[string]string),
			},
		},
		itemInput:   "item",
		subVertices: make(map[string]*config.VertexConfig),
	}

	mcb.CollectInto("out", "results1")
	vb := mcb.CollectInto("out", "results2") // second call — must be an error
	if vb.err == nil {
		t.Error("expected error for second MapConfigBuilder.CollectInto call, got nil")
	}
}

// TestBuilder_SubgraphCollectInto_DoubleCallError verifies the same guard on
// the nested SubgraphMapConfigBuilder (BUG-02 fix).
func TestBuilder_SubgraphCollectInto_DoubleCallError(t *testing.T) {
	smcb := &SubgraphMapConfigBuilder{
		svb: &SubgraphVertexBuilder{
			mcb: &MapConfigBuilder{
				vb: &VertexBuilder{
					config: &config.VertexConfig{
						Inputs:  make(map[string]string),
						Outputs: make(map[string]string),
					},
				},
				subVertices: make(map[string]*config.VertexConfig),
			},
			cfg: &config.VertexConfig{
				Inputs:  make(map[string]string),
				Outputs: make(map[string]string),
			},
		},
		itemInput:   "item",
		subVertices: make(map[string]*config.VertexConfig),
	}

	smcb.CollectInto("out", "results1")
	svb := smcb.CollectInto("out", "results2") // second call — must be an error
	if svb.err == nil {
		t.Error("expected error for second SubgraphMapConfigBuilder.CollectInto call, got nil")
	}
}

// TestBuilder_CollectInto_WritesResultsWire verifies that CollectInto stores the
// output wire in MapConfig.ResultsWire and does NOT write to Outputs["Results"] (BUG-02 fix).
func TestBuilder_CollectInto_WritesResultsWire(t *testing.T) {
	b := NewBuilder("results_wire_test").
		Vertex("source").
		Op("SourceOp").
		Output("Items", "list_wire").
		Vertex("map_v").
		Input("Items", "list_wire").
		MapOver("item").
		SubVertex("inner").
		Op("InnerOp").
		Output("Out", "inner_out").
		CollectInto("inner_out", "my_results")

	mapVB := b.builder.vertices["map_v"]
	if mapVB == nil {
		t.Fatal("map_v vertex not found in builder")
	}

	if mapVB.config.Map == nil {
		t.Fatal("map_v: config.Map is nil after CollectInto")
	}
	if mapVB.config.Map.ResultsWire != "my_results" {
		t.Errorf("Map.ResultsWire = %q, want %q", mapVB.config.Map.ResultsWire, "my_results")
	}
	if _, exists := mapVB.config.Outputs["Results"]; exists {
		t.Error("Outputs[\"Results\"] must not be set after CollectInto (BUG-02 fix)")
	}
}

// TestBuilder_SubgraphConditionalMethods verifies that Condition, ConditionInput,
// PassthroughWire, Merge, and OnError are available on SubgraphVertexBuilder
// (one-deep map sub-graph).
func TestBuilder_SubgraphConditionalMethods(t *testing.T) {
	g, err := NewBuilder("subgraph_cond_test").
		Vertex("source").
		Op("SourceOp").
		Output("Items", "list_wire").
		Output("Flag", "flag_wire").
		Vertex("map_v").
		Input("Items", "list_wire").
		MapOver("item").
		SubVertex("cond_v").
		Op("ProcessOp").
		Input("In", "item").
		Condition("is_positive").
		ConditionInput("item").
		PassthroughWire("Out", "item").
		Merge("coalesce").
		OnError("continue").
		Output("Out", "result").
		CollectInto("result", "results").
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapV := g.VertexByName("map_v")
	if mapV == nil || mapV.Map == nil {
		t.Fatal("map_v not found or Map config is nil")
	}

	subCfg, ok := mapV.Map.Subgraph.Vertices["cond_v"]
	if !ok {
		t.Fatal("sub-graph vertex 'cond_v' not found")
	}

	if subCfg.Condition != "is_positive" {
		t.Errorf("Condition = %q, want %q", subCfg.Condition, "is_positive")
	}
	if len(subCfg.ConditionInputs) != 1 || subCfg.ConditionInputs[0] != "item" {
		t.Errorf("ConditionInputs = %v, want [item]", subCfg.ConditionInputs)
	}
	if subCfg.PassthroughWires["Out"] != "item" {
		t.Errorf("PassthroughWires[Out] = %q, want %q", subCfg.PassthroughWires["Out"], "item")
	}
	if subCfg.Merge != "coalesce" {
		t.Errorf("Merge = %q, want %q", subCfg.Merge, "coalesce")
	}
	if subCfg.OnError != "continue" {
		t.Errorf("OnError = %q, want %q", subCfg.OnError, "continue")
	}
}

// TestBuilder_NestedSubgraphConditionalMethods verifies that Condition,
// ConditionInput, PassthroughWire, Merge, and OnError are available on
// NestedSubgraphVertexBuilder (two-deep map sub-graph).
func TestBuilder_NestedSubgraphConditionalMethods(t *testing.T) {
	g, err := NewBuilder("nested_subgraph_cond_test").
		Vertex("source").
		Op("SourceOp").
		Output("Items", "outer_list").
		Vertex("outer_map").
		Input("Items", "outer_list").
		MapOver("outer_item").
		SubVertex("inner_map").
		Input("Items", "outer_item").
		MapOver("inner_item").
		SubVertex("cond_inner").
		Op("ProcessOp").
		Input("In", "inner_item").
		Condition("is_negative").
		ConditionInput("inner_item").
		PassthroughWire("Out", "inner_item").
		Merge("coalesce").
		OnError("continue").
		Output("Out", "inner_result").
		CollectInto("inner_result", "inner_results").
		CollectInto("inner_results", "outer_results").
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outerMap := g.VertexByName("outer_map")
	if outerMap == nil || outerMap.Map == nil {
		t.Fatal("outer_map not found or Map config is nil")
	}

	innerMapCfg, ok := outerMap.Map.Subgraph.Vertices["inner_map"]
	if !ok {
		t.Fatal("sub-graph vertex 'inner_map' not found")
	}
	if innerMapCfg.Map == nil {
		t.Fatal("inner_map: expected Map config")
	}

	condInner, ok := innerMapCfg.Map.Subgraph.Vertices["cond_inner"]
	if !ok {
		t.Fatal("nested sub-graph vertex 'cond_inner' not found")
	}

	if condInner.Condition != "is_negative" {
		t.Errorf("Condition = %q, want %q", condInner.Condition, "is_negative")
	}
	if len(condInner.ConditionInputs) != 1 || condInner.ConditionInputs[0] != "inner_item" {
		t.Errorf("ConditionInputs = %v, want [inner_item]", condInner.ConditionInputs)
	}
	if condInner.PassthroughWires["Out"] != "inner_item" {
		t.Errorf("PassthroughWires[Out] = %q, want %q", condInner.PassthroughWires["Out"], "inner_item")
	}
	if condInner.Merge != "coalesce" {
		t.Errorf("Merge = %q, want %q", condInner.Merge, "coalesce")
	}
	if condInner.OnError != "continue" {
		t.Errorf("OnError = %q, want %q", condInner.OnError, "continue")
	}
}

// TestBuilder_SubgraphConditionalMethods_ValidationErrors verifies that empty
// wire names are rejected by the new methods on both sub-graph builder types.
func TestBuilder_SubgraphConditionalMethods_ValidationErrors(t *testing.T) {
	t.Run("SubgraphVertexBuilder_ConditionInput_empty", func(t *testing.T) {
		mcb := &MapConfigBuilder{
			vb:          &VertexBuilder{config: &config.VertexConfig{Inputs: make(map[string]string), Outputs: make(map[string]string)}},
			itemInput:   "item",
			subVertices: make(map[string]*config.VertexConfig),
		}
		svb := mcb.SubVertex("v")
		svb.ConditionInput("")
		if svb.err == nil {
			t.Error("expected error for empty ConditionInput wire, got nil")
		}
	})

	t.Run("SubgraphVertexBuilder_PassthroughWire_empty", func(t *testing.T) {
		mcb := &MapConfigBuilder{
			vb:          &VertexBuilder{config: &config.VertexConfig{Inputs: make(map[string]string), Outputs: make(map[string]string)}},
			itemInput:   "item",
			subVertices: make(map[string]*config.VertexConfig),
		}
		svb := mcb.SubVertex("v")
		svb.PassthroughWire("", "src")
		if svb.err == nil {
			t.Error("expected error for empty PassthroughWire outputField, got nil")
		}
	})

	t.Run("NestedSubgraphVertexBuilder_ConditionInput_empty", func(t *testing.T) {
		smcb := &SubgraphMapConfigBuilder{
			svb: &SubgraphVertexBuilder{
				mcb: &MapConfigBuilder{
					vb:          &VertexBuilder{config: &config.VertexConfig{Inputs: make(map[string]string), Outputs: make(map[string]string)}},
					subVertices: make(map[string]*config.VertexConfig),
				},
				cfg: &config.VertexConfig{Inputs: make(map[string]string), Outputs: make(map[string]string)},
			},
			itemInput:   "item",
			subVertices: make(map[string]*config.VertexConfig),
		}
		nsvb := smcb.SubVertex("v")
		nsvb.ConditionInput("")
		if nsvb.err == nil {
			t.Error("expected error for empty ConditionInput wire, got nil")
		}
	})

	t.Run("NestedSubgraphVertexBuilder_PassthroughWire_empty", func(t *testing.T) {
		smcb := &SubgraphMapConfigBuilder{
			svb: &SubgraphVertexBuilder{
				mcb: &MapConfigBuilder{
					vb:          &VertexBuilder{config: &config.VertexConfig{Inputs: make(map[string]string), Outputs: make(map[string]string)}},
					subVertices: make(map[string]*config.VertexConfig),
				},
				cfg: &config.VertexConfig{Inputs: make(map[string]string), Outputs: make(map[string]string)},
			},
			itemInput:   "item",
			subVertices: make(map[string]*config.VertexConfig),
		}
		nsvb := smcb.SubVertex("v")
		nsvb.PassthroughWire("out", "")
		if nsvb.err == nil {
			t.Error("expected error for empty PassthroughWire sourceWire, got nil")
		}
	})
}

// TestBuilder_SubgraphDuplicateVertexName verifies that defining a sub-graph
// vertex name twice in a MapOver chain produces an error from Build().
func TestBuilder_SubgraphDuplicateVertexName(t *testing.T) {
	_, err := NewBuilder("dup_subgraph_test").
		Vertex("source").Op("SourceOp").Output("Items", "list").
		Vertex("map_v").
		Input("Items", "list").
		MapOver("item").
		SubVertex("x").Op("OpA").Output("Out", "res").
		SubVertex("x").Op("OpB").Output("Out", "res"). // duplicate
		CollectInto("res", "results").
		Build()

	if err == nil {
		t.Fatal("expected error for duplicate sub-graph vertex name, got nil")
	}
	if !strings.Contains(err.Error(), `"x"`) {
		t.Errorf("error should mention the duplicate name, got: %v", err)
	}
}

// TestBuilder_NestedSubgraphDuplicateVertexName verifies the same guard on the
// nested (two-deep) SubgraphMapConfigBuilder.
func TestBuilder_NestedSubgraphDuplicateVertexName(t *testing.T) {
	_, err := NewBuilder("dup_nested_subgraph_test").
		Vertex("source").Op("SourceOp").Output("Items", "list").
		Vertex("outer_map").
		Input("Items", "list").
		MapOver("outer_item").
		SubVertex("inner_map").
		Input("Items", "outer_item").
		MapOver("inner_item").
		SubVertex("x").Op("OpA").Output("Out", "res").
		SubVertex("x").Op("OpB").Output("Out", "res"). // duplicate in nested sub-graph
		CollectInto("res", "inner_results").
		CollectInto("inner_results", "results").
		Build()

	if err == nil {
		t.Fatal("expected error for duplicate nested sub-graph vertex name, got nil")
	}
	if !strings.Contains(err.Error(), `"x"`) {
		t.Errorf("error should mention the duplicate name, got: %v", err)
	}
}

func TestBuilder_BulkMappingsAndRawParams(t *testing.T) {
	raw := []byte(`{"key": "value"}`)
	g, err := NewBuilder("bulk_test").
		Vertex("p1").Op("OpSource").Output("outA", "wireA").Done().
		Vertex("p2").Op("OpSource").Output("outB", "wireB").Done().
		Vertex("v1").
		Op("Op1").
		Inputs(map[string]string{"a": "wireA", "b": "wireB"}).
		Outputs(map[string]string{"out": "wireOut"}).
		RawParams(raw).
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v := g.VertexByName("v1")
	if v.Inputs["a"] != "wireA" || v.Inputs["b"] != "wireB" {
		t.Errorf("bulk inputs mapping failed: %v", v.Inputs)
	}
	if string(v.Params().GetRaw()) != string(raw) {
		t.Errorf("RawParams failed: expected %s, got %s", string(raw), string(v.Params().GetRaw()))
	}
}

func TestBuilder_FilterBy_Basic(t *testing.T) {
	g, err := NewBuilder("filter_graph").
		Vertex("source").
		Op("SourceOp").
		Output("out", "source_wire").
		Vertex("filter").
		Input("In", "source_wire").
		FilterBy("positive").
		CollectInto("filtered_wire").
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v := g.VertexByName("filter")
	if v == nil {
		t.Fatal("vertex 'filter' not found")
	}
	if v.Filter == nil {
		t.Fatal("expected non-nil Filter on vertex")
	}
	if v.Filter.Predicate != "positive" {
		t.Errorf("expected Predicate 'positive', got %q", v.Filter.Predicate)
	}
	if v.Filter.ResultsWire != "filtered_wire" {
		t.Errorf("expected ResultsWire 'filtered_wire', got %q", v.Filter.ResultsWire)
	}
	if v.Filter.ItemKey != "" {
		t.Errorf("expected empty ItemKey (default), got %q", v.Filter.ItemKey)
	}
}

func TestBuilder_FilterBy_ItemKey(t *testing.T) {
	g, err := NewBuilder("filter_graph").
		Vertex("source").
		Op("SourceOp").
		Output("out", "source_wire").
		Vertex("filter").
		Input("In", "source_wire").
		FilterBy("positive").
		ItemKey("element").
		CollectInto("filtered_wire").
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v := g.VertexByName("filter")
	if v.Filter.ItemKey != "element" {
		t.Errorf("expected ItemKey 'element', got %q", v.Filter.ItemKey)
	}
}

func TestBuilder_FilterBy_EmptyPredicate_Error(t *testing.T) {
	_, err := NewBuilder("filter_graph").
		Vertex("source").
		Op("SourceOp").
		Output("out", "source_wire").
		Vertex("filter").
		Input("In", "source_wire").
		FilterBy("").
		CollectInto("filtered_wire").
		Build()

	if err == nil {
		t.Fatal("expected error for empty predicateName, got nil")
	}
}

func TestBuilder_FilterBy_EmptyOutputWire_Error(t *testing.T) {
	_, err := NewBuilder("filter_graph").
		Vertex("source").
		Op("SourceOp").
		Output("out", "source_wire").
		Vertex("filter").
		Input("In", "source_wire").
		FilterBy("positive").
		CollectInto("").
		Build()

	if err == nil {
		t.Fatal("expected error for empty outputWire, got nil")
	}
}

func TestBuilder_FilterBy_EmptyItemKey_Error(t *testing.T) {
	_, err := NewBuilder("filter_graph").
		Vertex("source").
		Op("SourceOp").
		Output("out", "source_wire").
		Vertex("filter").
		Input("In", "source_wire").
		FilterBy("positive").
		ItemKey("").
		CollectInto("filtered_wire").
		Build()

	if err == nil {
		t.Fatal("expected error for empty ItemKey, got nil")
	}
}

func TestBuilder_FilterBy_CollectInto_Twice_Error(t *testing.T) {
	_, err := NewBuilder("filter_graph").
		Vertex("source").
		Op("SourceOp").
		Output("out", "source_wire").
		Vertex("filter").
		Input("In", "source_wire").
		FilterBy("positive").
		CollectInto("filtered_wire").
		FilterBy("negative").
		CollectInto("other_wire").
		Build()

	if err == nil {
		t.Fatal("expected error when CollectInto called twice, got nil")
	}
}
