package config

import "encoding/json"

type GraphConfig struct {
	Name     string                   `json:"name"`
	Vertices map[string]*VertexConfig `json:"vertices"`

	// ExternalWires lists wire names that are injected into the graph from
	// outside (e.g. by a map node). No DAG edge is created for these wires;
	// they must be pre-seeded in the engine status before Run is called.
	ExternalWires []string `json:"external_wires,omitempty"`
}

// FilterConfig configures element-wise predicate filtering over a slice input.
// The engine applies the named predicate to each element and collects only the
// elements for which the predicate returns true into a []any output slice.
//
// Wire convention: each element is wrapped in a *T pointer before being passed
// as the predicate's ItemKey input, matching dagor's pointer-based wire passing.
// Predicates that filter a []int slice should therefore declare their input as
// *int and dereference in the predicate body.
type FilterConfig struct {
	// Predicate is the registered predicate name applied to each element.
	Predicate string `json:"predicate"`

	// ItemKey is the key used in the predicate inputs map for each element.
	// Defaults to "item" when empty.
	ItemKey string `json:"item_key,omitempty"`

	// ResultsWire is the wire in the parent graph where the kept []any slice
	// is written. Set by the builder via CollectInto.
	ResultsWire string `json:"results_wire"`
}

// MapConfig configures fan-out execution of a sub-graph over every element of
// a slice input. The engine runs one sub-graph instance per element using the
// shared goroutine pool, then assembles the per-element results into a []any
// output slice on the map vertex's output wire.
//
// Wire convention: each element is wrapped in a pointer (*T) before being
// injected as ItemInput, matching dagor's standard pointer-based wire passing.
// Sub-graph operators that read ItemInput should therefore declare their input
// field as *T and type-assert in SetInputField.
type MapConfig struct {
	// ItemInput is the wire name inside Subgraph that receives each element as
	// a *T pointer. It must be listed in Subgraph.ExternalWires so the graph
	// builder does not require a producer vertex for it.
	ItemInput string `json:"item_input"`

	// ResultOutput is the wire name inside Subgraph whose value is collected
	// as the per-element result. Pointer values are automatically dereferenced
	// before appending to the []any results slice.
	ResultOutput string `json:"result_output"`

	// ResultsWire is the wire name in the parent graph where the assembled
	// []any results slice is written. Set by the builder via CollectInto;
	// stored here rather than in VertexConfig.Outputs to avoid the magic "Results" key.
	ResultsWire string `json:"results_wire,omitempty"`

	// Subgraph is the graph definition executed once per input element.
	// It must declare ItemInput in its ExternalWires list.
	Subgraph *GraphConfig `json:"subgraph"`
}

// OnError is the action to take when an error occurs.
const (
	OnErrorStop     string = "stop"     // stop the graph execution when a vertex error occurs.
	OnErrorContinue string = "continue" // continue graph execution when a vertex errors: the vertex is
	// marked skipped and its outputs are cleared (or passthrough-filled), so
	// successors see nil and propagate the skip transitively.
)

type VertexConfig struct {
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params"`

	// input fields map. operator field name -> vertex field name.
	Inputs map[string]string `json:"inputs"`
	// output fields map. operator field name -> vertex field name.
	Outputs map[string]string `json:"outputs"`

	// on error action.
	// default is "stop".
	OnError string `json:"on_error"`

	// condition is an optional registered predicate name.
	// If empty, the vertex always executes.
	Condition string `json:"condition"`

	// ConditionInputs are wire names needed by the Condition predicate but NOT
	// fed to the op. The engine creates a real DAG edge for each wire so the
	// producer runs before this vertex is evaluated.
	ConditionInputs []string `json:"condition_inputs,omitempty"`

	// PassthroughWires maps op output field name → source wire name. When the
	// vertex is skipped the engine sets the output to that wire's value instead
	// of nil, enabling value inheritance without embedding skip logic in the op.
	PassthroughWires map[string]string `json:"passthrough_wires,omitempty"`

	// merge controls skip-propagation when a vertex has inputs from multiple
	// branches. MergeCoalesce skips only when ALL input-producing vertices were
	// skipped, and tolerates missing field values (treating them as nil).
	// Leave empty for the default "skip if any producer was skipped" behaviour.
	Merge string `json:"merge,omitempty"`

	// Map configures this vertex as a map node that fans out over a slice
	// input. Mutually exclusive with Op and Filter.
	Map *MapConfig `json:"map,omitempty"`

	// Filter configures this vertex as a filter node that applies a predicate
	// to each element of a slice input, keeping only matching elements.
	// Mutually exclusive with Op, Map, and Reduce.
	Filter *FilterConfig `json:"filter,omitempty"`

	// Reduce configures this vertex as a reduce node that folds a slice into a
	// single accumulated value using a registered reducer function.
	// Mutually exclusive with Op, Map, and Filter.
	Reduce *ReduceConfig `json:"reduce,omitempty"`
}

// ReduceConfig configures element-wise fold/reduce over a slice input.
// The engine applies the named reducer to fold the slice into a single value,
// starting from either the first element (when InitWire is empty) or a value
// supplied by another vertex via InitWire.
//
// Wire convention: elements are passed as their concrete (non-pointer) values
// to the reducer, matching the values stored in the []any slice produced by a
// map or filter vertex. The accumulated result is written directly (no
// extra pointer wrapping) to ResultsWire.
type ReduceConfig struct {
	// Reducer is the registered reducer function name.
	Reducer string `json:"reducer"`

	// InitWire is an optional wire name for the initial accumulator value.
	// When set, the engine reads the wire's value before iterating the slice.
	// When empty, the first element of the slice is used as the initial
	// accumulator and reduction starts from the second element. An empty
	// input slice with no InitWire produces a nil result.
	InitWire string `json:"init_wire,omitempty"`

	// ResultsWire is the wire in the parent graph where the final accumulated
	// value is written. Set by the builder via CollectInto.
	ResultsWire string `json:"results_wire"`
}

// MergeCoalesce is the merge strategy that skips only when every input-producing
// vertex was skipped, enabling a single output node to sit downstream of two
// mutually-exclusive conditional branches.
const MergeCoalesce = "coalesce"
