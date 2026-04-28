package graph

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wwz16/dagor/config"
)

// MapConfigBuilder is a fluent builder for a map vertex's sub-graph.
// Obtain one via VertexBuilder.MapOver().
type MapConfigBuilder struct {
	vb          *VertexBuilder
	itemInput   string
	subVertices map[string]*config.VertexConfig
	err         error
}

// SubVertex starts a new vertex definition inside the sub-graph.
// If a previous SubVertex call is pending, it is committed first.
func (mcb *MapConfigBuilder) SubVertex(name string) *SubgraphVertexBuilder {
	return &SubgraphVertexBuilder{
		mcb:  mcb,
		name: name,
		cfg: &config.VertexConfig{
			Inputs:  make(map[string]string),
			Outputs: make(map[string]string),
		},
	}
}

// CollectInto finalizes the sub-graph configuration.
// resultOutput is the wire name inside the sub-graph whose value is collected
// per element. outputWire is the wire name in the parent graph where the
// assembled []any slice is written.
// Returns the parent VertexBuilder so the fluent chain can continue.
func (mcb *MapConfigBuilder) CollectInto(resultOutput, outputWire string) *VertexBuilder {
	if resultOutput == "" || outputWire == "" {
		mcb.err = errors.Join(mcb.err, fmt.Errorf("MapConfigBuilder: CollectInto requires non-empty resultOutput and outputWire"))
	}
	if mcb.vb.config.Map != nil {
		mcb.err = errors.Join(mcb.err, fmt.Errorf("MapConfigBuilder: CollectInto called more than once"))
	}
	mcb.vb.config.Map = &config.MapConfig{
		ItemInput:    mcb.itemInput,
		ResultOutput: resultOutput,
		ResultsWire:  outputWire,
		Subgraph: &config.GraphConfig{
			ExternalWires: []string{mcb.itemInput},
			Vertices:      mcb.subVertices,
		},
	}
	if mcb.err != nil {
		mcb.vb.err = errors.Join(mcb.vb.err, mcb.err)
	}
	return mcb.vb
}

// SubgraphVertexBuilder is a fluent builder for a single vertex inside a map
// sub-graph. Methods mirror VertexBuilder for consistency.
type SubgraphVertexBuilder struct {
	mcb  *MapConfigBuilder
	name string
	cfg  *config.VertexConfig
	err  error
}

// Op sets the operator name.
func (svb *SubgraphVertexBuilder) Op(op string) *SubgraphVertexBuilder {
	svb.cfg.Op = op
	return svb
}

// Params sets the operator parameters.
func (svb *SubgraphVertexBuilder) Params(p any) *SubgraphVertexBuilder {
	raw, err := json.Marshal(p)
	if err != nil {
		svb.err = errors.Join(svb.err, fmt.Errorf("subgraph vertex %q: marshal params: %w", svb.name, err))
		return svb
	}
	svb.cfg.Params = raw
	return svb
}

// Input adds an input field mapping (opField → wire).
func (svb *SubgraphVertexBuilder) Input(opField, wire string) *SubgraphVertexBuilder {
	svb.cfg.Inputs[opField] = wire
	return svb
}

// Output adds an output field mapping (opField → wire).
func (svb *SubgraphVertexBuilder) Output(opField, wire string) *SubgraphVertexBuilder {
	svb.cfg.Outputs[opField] = wire
	return svb
}

// Condition sets the predicate name for the vertex.
func (svb *SubgraphVertexBuilder) Condition(condition string) *SubgraphVertexBuilder {
	svb.cfg.Condition = condition
	return svb
}

// ConditionInput declares a wire needed by the Condition predicate but not fed
// to the op. Empty wire names are rejected.
func (svb *SubgraphVertexBuilder) ConditionInput(wire string) *SubgraphVertexBuilder {
	if wire == "" {
		svb.err = errors.Join(svb.err, fmt.Errorf("subgraph vertex %q: condition input wire name cannot be empty", svb.name))
		return svb
	}
	svb.cfg.ConditionInputs = append(svb.cfg.ConditionInputs, wire)
	return svb
}

// PassthroughWire declares that when this vertex is skipped, outputField should
// be set to the value of sourceWire. Empty field names are rejected.
func (svb *SubgraphVertexBuilder) PassthroughWire(outputField, sourceWire string) *SubgraphVertexBuilder {
	if outputField == "" || sourceWire == "" {
		svb.err = errors.Join(svb.err, fmt.Errorf("subgraph vertex %q: passthrough wire cannot have empty fields (outputField=%q, sourceWire=%q)", svb.name, outputField, sourceWire))
		return svb
	}
	if svb.cfg.PassthroughWires == nil {
		svb.cfg.PassthroughWires = make(map[string]string)
	}
	svb.cfg.PassthroughWires[outputField] = sourceWire
	return svb
}

// Merge sets the merge strategy for the vertex.
func (svb *SubgraphVertexBuilder) Merge(merge string) *SubgraphVertexBuilder {
	svb.cfg.Merge = merge
	return svb
}

// OnError sets the error action for the vertex.
func (svb *SubgraphVertexBuilder) OnError(action string) *SubgraphVertexBuilder {
	svb.cfg.OnError = action
	return svb
}

// MapOver configures this sub-graph vertex as a nested map node that fans out
// over a slice input. Returns a SubgraphMapConfigBuilder to define the nested
// sub-graph vertices; call CollectInto on it to finalize and return here.
func (svb *SubgraphVertexBuilder) MapOver(itemInput string) *SubgraphMapConfigBuilder {
	if itemInput == "" {
		svb.err = errors.Join(svb.err, fmt.Errorf("subgraph vertex %q: MapOver itemInput cannot be empty", svb.name))
	}
	return &SubgraphMapConfigBuilder{
		svb:         svb,
		itemInput:   itemInput,
		subVertices: make(map[string]*config.VertexConfig),
	}
}

// SubVertex commits this vertex and starts the next sub-graph vertex.
func (svb *SubgraphVertexBuilder) SubVertex(name string) *SubgraphVertexBuilder {
	svb.commit()
	return svb.mcb.SubVertex(name)
}

// CollectInto commits this vertex and finalizes the sub-graph configuration.
func (svb *SubgraphVertexBuilder) CollectInto(resultOutput, outputWire string) *VertexBuilder {
	svb.commit()
	return svb.mcb.CollectInto(resultOutput, outputWire)
}

func (svb *SubgraphVertexBuilder) commit() {
	if svb.err != nil {
		svb.mcb.err = errors.Join(svb.mcb.err, svb.err)
	}
	if _, exists := svb.mcb.subVertices[svb.name]; exists {
		svb.mcb.err = errors.Join(svb.mcb.err, fmt.Errorf("sub-graph vertex %q defined more than once", svb.name))
		return
	}
	svb.mcb.subVertices[svb.name] = svb.cfg
}

// SubgraphMapConfigBuilder builds the sub-graph for a map vertex that is
// itself defined inside another map's sub-graph. Obtain one via
// SubgraphVertexBuilder.MapOver().
type SubgraphMapConfigBuilder struct {
	svb         *SubgraphVertexBuilder
	itemInput   string
	subVertices map[string]*config.VertexConfig
	err         error
}

// SubVertex starts a new vertex definition inside the nested sub-graph.
func (smcb *SubgraphMapConfigBuilder) SubVertex(name string) *NestedSubgraphVertexBuilder {
	return &NestedSubgraphVertexBuilder{
		smcb: smcb,
		name: name,
		cfg: &config.VertexConfig{
			Inputs:  make(map[string]string),
			Outputs: make(map[string]string),
		},
	}
}

// CollectInto finalizes the nested sub-graph and returns the parent
// SubgraphVertexBuilder so the fluent chain can continue.
func (smcb *SubgraphMapConfigBuilder) CollectInto(resultOutput, outputWire string) *SubgraphVertexBuilder {
	if resultOutput == "" || outputWire == "" {
		smcb.err = errors.Join(smcb.err, fmt.Errorf("SubgraphMapConfigBuilder: CollectInto requires non-empty resultOutput and outputWire"))
	}
	if smcb.svb.cfg.Map != nil {
		smcb.err = errors.Join(smcb.err, fmt.Errorf("SubgraphMapConfigBuilder: CollectInto called more than once"))
	}
	smcb.svb.cfg.Map = &config.MapConfig{
		ItemInput:    smcb.itemInput,
		ResultOutput: resultOutput,
		ResultsWire:  outputWire,
		Subgraph: &config.GraphConfig{
			ExternalWires: []string{smcb.itemInput},
			Vertices:      smcb.subVertices,
		},
	}
	if smcb.err != nil {
		smcb.svb.err = errors.Join(smcb.svb.err, smcb.err)
	}
	return smcb.svb
}

// NestedSubgraphVertexBuilder builds a single vertex inside a doubly-nested
// map sub-graph. Methods mirror SubgraphVertexBuilder for consistency.
type NestedSubgraphVertexBuilder struct {
	smcb *SubgraphMapConfigBuilder
	name string
	cfg  *config.VertexConfig
	err  error
}

// Op sets the operator name.
func (nsvb *NestedSubgraphVertexBuilder) Op(op string) *NestedSubgraphVertexBuilder {
	nsvb.cfg.Op = op
	return nsvb
}

// Params sets the operator parameters.
func (nsvb *NestedSubgraphVertexBuilder) Params(p any) *NestedSubgraphVertexBuilder {
	raw, err := json.Marshal(p)
	if err != nil {
		nsvb.err = errors.Join(nsvb.err, fmt.Errorf("nested subgraph vertex %q: marshal params: %w", nsvb.name, err))
		return nsvb
	}
	nsvb.cfg.Params = raw
	return nsvb
}

// Input adds an input field mapping (opField → wire).
func (nsvb *NestedSubgraphVertexBuilder) Input(opField, wire string) *NestedSubgraphVertexBuilder {
	nsvb.cfg.Inputs[opField] = wire
	return nsvb
}

// Output adds an output field mapping (opField → wire).
func (nsvb *NestedSubgraphVertexBuilder) Output(opField, wire string) *NestedSubgraphVertexBuilder {
	nsvb.cfg.Outputs[opField] = wire
	return nsvb
}

// Condition sets the predicate name for the vertex.
func (nsvb *NestedSubgraphVertexBuilder) Condition(condition string) *NestedSubgraphVertexBuilder {
	nsvb.cfg.Condition = condition
	return nsvb
}

// ConditionInput declares a wire needed by the Condition predicate but not fed
// to the op. Empty wire names are rejected.
func (nsvb *NestedSubgraphVertexBuilder) ConditionInput(wire string) *NestedSubgraphVertexBuilder {
	if wire == "" {
		nsvb.err = errors.Join(nsvb.err, fmt.Errorf("nested subgraph vertex %q: condition input wire name cannot be empty", nsvb.name))
		return nsvb
	}
	nsvb.cfg.ConditionInputs = append(nsvb.cfg.ConditionInputs, wire)
	return nsvb
}

// PassthroughWire declares that when this vertex is skipped, outputField should
// be set to the value of sourceWire. Empty field names are rejected.
func (nsvb *NestedSubgraphVertexBuilder) PassthroughWire(outputField, sourceWire string) *NestedSubgraphVertexBuilder {
	if outputField == "" || sourceWire == "" {
		nsvb.err = errors.Join(nsvb.err, fmt.Errorf("nested subgraph vertex %q: passthrough wire cannot have empty fields (outputField=%q, sourceWire=%q)", nsvb.name, outputField, sourceWire))
		return nsvb
	}
	if nsvb.cfg.PassthroughWires == nil {
		nsvb.cfg.PassthroughWires = make(map[string]string)
	}
	nsvb.cfg.PassthroughWires[outputField] = sourceWire
	return nsvb
}

// Merge sets the merge strategy for the vertex.
func (nsvb *NestedSubgraphVertexBuilder) Merge(merge string) *NestedSubgraphVertexBuilder {
	nsvb.cfg.Merge = merge
	return nsvb
}

// OnError sets the error action for the vertex.
func (nsvb *NestedSubgraphVertexBuilder) OnError(action string) *NestedSubgraphVertexBuilder {
	nsvb.cfg.OnError = action
	return nsvb
}

// SubVertex commits this vertex and starts the next nested sub-graph vertex.
func (nsvb *NestedSubgraphVertexBuilder) SubVertex(name string) *NestedSubgraphVertexBuilder {
	nsvb.commit()
	return nsvb.smcb.SubVertex(name)
}

// CollectInto commits this vertex and finalizes the nested sub-graph configuration.
func (nsvb *NestedSubgraphVertexBuilder) CollectInto(resultOutput, outputWire string) *SubgraphVertexBuilder {
	nsvb.commit()
	return nsvb.smcb.CollectInto(resultOutput, outputWire)
}

func (nsvb *NestedSubgraphVertexBuilder) commit() {
	if nsvb.err != nil {
		nsvb.smcb.err = errors.Join(nsvb.smcb.err, nsvb.err)
	}
	if _, exists := nsvb.smcb.subVertices[nsvb.name]; exists {
		nsvb.smcb.err = errors.Join(nsvb.smcb.err, fmt.Errorf("nested sub-graph vertex %q defined more than once", nsvb.name))
		return
	}
	nsvb.smcb.subVertices[nsvb.name] = nsvb.cfg
}

// Builder is a fluent builder for Graph.
type Builder struct {
	name     string
	vertices map[string]*VertexBuilder
	err      error
}

// NewBuilder creates a new Graph builder.
func NewBuilder(name string) *Builder {
	return &Builder{
		name:     name,
		vertices: make(map[string]*VertexBuilder),
	}
}

// Vertex adds or returns a vertex builder for the named vertex.
func (b *Builder) Vertex(name string) *VertexBuilder {
	if name == "" {
		b.err = errors.Join(b.err, fmt.Errorf("graph %q: vertex name cannot be empty", b.name))
		return &VertexBuilder{builder: b, name: "unknown", config: &config.VertexConfig{}}
	}
	if vb, ok := b.vertices[name]; ok {
		return vb
	}
	vb := &VertexBuilder{
		builder: b,
		name:    name,
		config: &config.VertexConfig{
			Inputs:  make(map[string]string),
			Outputs: make(map[string]string),
		},
	}
	b.vertices[name] = vb
	return vb
}

// Build constructs the Graph from the builder's state.
func (b *Builder) Build() (*Graph, error) {
	var allErrors error
	if b.err != nil {
		allErrors = errors.Join(allErrors, b.err)
	}

	cfg := &config.GraphConfig{
		Name:     b.name,
		Vertices: make(map[string]*config.VertexConfig),
	}

	for name, vb := range b.vertices {
		if vb.err != nil {
			allErrors = errors.Join(allErrors, vb.err)
		}
		cfg.Vertices[name] = vb.config
	}

	if allErrors != nil {
		return nil, allErrors
	}

	return NewGraphFromConfig(cfg)
}

// VertexBuilder is a fluent builder for a single vertex.
type VertexBuilder struct {
	builder *Builder
	name    string
	config  *config.VertexConfig
	err     error
}

// Op sets the operator name for the vertex.
func (vb *VertexBuilder) Op(op string) *VertexBuilder {
	if op == "" {
		vb.err = errors.Join(vb.err, fmt.Errorf("vertex %q: operator name cannot be empty", vb.name))
		return vb
	}
	vb.config.Op = op
	return vb
}

// Params sets the parameters for the vertex. It marshals the provided value to JSON.
func (vb *VertexBuilder) Params(p any) *VertexBuilder {
	if p == nil {
		return vb
	}
	raw, err := json.Marshal(p)
	if err != nil {
		vb.err = errors.Join(vb.err, fmt.Errorf("vertex %q: failed to marshal params: %w", vb.name, err))
		return vb
	}
	vb.config.Params = raw
	return vb
}

// RawParams sets the parameters for the vertex using pre-marshaled JSON.
func (vb *VertexBuilder) RawParams(raw json.RawMessage) *VertexBuilder {
	vb.config.Params = raw
	return vb
}

// Inputs sets the input fields mapping.
func (vb *VertexBuilder) Inputs(inputs map[string]string) *VertexBuilder {
	for k, v := range inputs {
		vb.config.Inputs[k] = v
	}
	return vb
}

// Input adds a single input field mapping.
func (vb *VertexBuilder) Input(opField, vertexField string) *VertexBuilder {
	if opField == "" || vertexField == "" {
		vb.err = errors.Join(vb.err, fmt.Errorf("vertex %q: input mapping cannot have empty fields (opField=%q, vertexField=%q)", vb.name, opField, vertexField))
		return vb
	}
	vb.config.Inputs[opField] = vertexField
	return vb
}

// Outputs sets the output fields mapping.
func (vb *VertexBuilder) Outputs(outputs map[string]string) *VertexBuilder {
	for k, v := range outputs {
		vb.config.Outputs[k] = v
	}
	return vb
}

// Output adds a single output field mapping.
func (vb *VertexBuilder) Output(opField, vertexField string) *VertexBuilder {
	if opField == "" || vertexField == "" {
		vb.err = errors.Join(vb.err, fmt.Errorf("vertex %q: output mapping cannot have empty fields (opField=%q, vertexField=%q)", vb.name, opField, vertexField))
		return vb
	}
	vb.config.Outputs[opField] = vertexField
	return vb
}

// Condition sets the predicate name for the vertex.
func (vb *VertexBuilder) Condition(condition string) *VertexBuilder {
	vb.config.Condition = condition
	return vb
}

// ConditionInput declares a wire that is needed by the Condition predicate but
// not fed to the op. The engine creates a DAG edge for it so the producer is
// guaranteed to run before this vertex is evaluated.
func (vb *VertexBuilder) ConditionInput(wire string) *VertexBuilder {
	if wire == "" {
		vb.err = errors.Join(vb.err, fmt.Errorf("vertex %q: condition input wire name cannot be empty", vb.name))
		return vb
	}
	vb.config.ConditionInputs = append(vb.config.ConditionInputs, wire)
	return vb
}

// PassthroughWire declares that when this vertex is skipped, outputField should
// be set to the value of sourceWire rather than nil.
func (vb *VertexBuilder) PassthroughWire(outputField, sourceWire string) *VertexBuilder {
	if outputField == "" || sourceWire == "" {
		vb.err = errors.Join(vb.err, fmt.Errorf("vertex %q: passthrough wire cannot have empty fields (outputField=%q, sourceWire=%q)", vb.name, outputField, sourceWire))
		return vb
	}
	if vb.config.PassthroughWires == nil {
		vb.config.PassthroughWires = make(map[string]string)
	}
	vb.config.PassthroughWires[outputField] = sourceWire
	return vb
}

// Merge sets the merge strategy for the vertex.
func (vb *VertexBuilder) Merge(merge string) *VertexBuilder {
	vb.config.Merge = merge
	return vb
}

// OnError sets the error action for the vertex.
func (vb *VertexBuilder) OnError(action string) *VertexBuilder {
	vb.config.OnError = action
	return vb
}

// FilterConfigBuilder is a fluent builder for a filter vertex.
// Obtain one via VertexBuilder.FilterBy().
type FilterConfigBuilder struct {
	vb        *VertexBuilder
	predicate string
	itemKey   string
	err       error
}

// ItemKey overrides the key used in the predicate inputs map for each element.
// Defaults to "item" when not set. Must be non-empty.
func (fcb *FilterConfigBuilder) ItemKey(key string) *FilterConfigBuilder {
	if key == "" {
		fcb.err = errors.Join(fcb.err, fmt.Errorf("FilterConfigBuilder: ItemKey cannot be empty"))
		return fcb
	}
	fcb.itemKey = key
	return fcb
}

// CollectInto finalizes the filter configuration.
// outputWire is the wire name in the parent graph where the kept []any slice
// is written. Returns the parent VertexBuilder so the fluent chain can continue.
func (fcb *FilterConfigBuilder) CollectInto(outputWire string) *VertexBuilder {
	if outputWire == "" {
		fcb.err = errors.Join(fcb.err, fmt.Errorf("FilterConfigBuilder: CollectInto requires non-empty outputWire"))
	}
	if fcb.vb.config.Filter != nil {
		fcb.err = errors.Join(fcb.err, fmt.Errorf("FilterConfigBuilder: CollectInto called more than once"))
	}
	fcb.vb.config.Filter = &config.FilterConfig{
		Predicate:   fcb.predicate,
		ItemKey:     fcb.itemKey,
		ResultsWire: outputWire,
	}
	if fcb.err != nil {
		fcb.vb.err = errors.Join(fcb.vb.err, fcb.err)
	}
	return fcb.vb
}

// FilterBy configures this vertex as a filter node that applies the named
// predicate to each element of the input slice, keeping only matching elements.
// The input slice wire must already be declared via Input().
// Returns a FilterConfigBuilder; call CollectInto on it to finalize and return here.
func (vb *VertexBuilder) FilterBy(predicateName string) *FilterConfigBuilder {
	if predicateName == "" {
		vb.err = errors.Join(vb.err, fmt.Errorf("vertex %q: FilterBy predicateName cannot be empty", vb.name))
	}
	return &FilterConfigBuilder{
		vb:        vb,
		predicate: predicateName,
	}
}

// ReduceConfigBuilder is a fluent builder for a reduce vertex.
// Obtain one via VertexBuilder.ReduceBy().
type ReduceConfigBuilder struct {
	vb       *VertexBuilder
	reducer  string
	initWire string
	err      error
}

// InitFrom sets the wire from which the initial accumulator value is read.
// When not called, the first element of the input slice is used as the seed.
// The wire must be non-empty.
func (rcb *ReduceConfigBuilder) InitFrom(wire string) *ReduceConfigBuilder {
	if wire == "" {
		rcb.err = errors.Join(rcb.err, fmt.Errorf("ReduceConfigBuilder: InitFrom wire cannot be empty"))
		return rcb
	}
	rcb.initWire = wire
	return rcb
}

// CollectInto finalizes the reduce configuration.
// outputWire is the wire name in the parent graph where the accumulated value
// is written. Returns the parent VertexBuilder so the fluent chain can continue.
func (rcb *ReduceConfigBuilder) CollectInto(outputWire string) *VertexBuilder {
	if outputWire == "" {
		rcb.err = errors.Join(rcb.err, fmt.Errorf("ReduceConfigBuilder: CollectInto requires non-empty outputWire"))
	}
	if rcb.vb.config.Reduce != nil {
		rcb.err = errors.Join(rcb.err, fmt.Errorf("ReduceConfigBuilder: CollectInto called more than once"))
	}
	rcb.vb.config.Reduce = &config.ReduceConfig{
		Reducer:     rcb.reducer,
		InitWire:    rcb.initWire,
		ResultsWire: outputWire,
	}
	if rcb.err != nil {
		rcb.vb.err = errors.Join(rcb.vb.err, rcb.err)
	}
	return rcb.vb
}

// ReduceBy configures this vertex as a reduce node that folds the input slice
// into a single value using the named reducer function. The input slice wire
// must already be declared via Input(). Returns a ReduceConfigBuilder; call
// CollectInto on it to finalize and return here.
func (vb *VertexBuilder) ReduceBy(reducerName string) *ReduceConfigBuilder {
	if reducerName == "" {
		vb.err = errors.Join(vb.err, fmt.Errorf("vertex %q: ReduceBy reducerName cannot be empty", vb.name))
	}
	return &ReduceConfigBuilder{
		vb:      vb,
		reducer: reducerName,
	}
}

// MapOver configures this vertex as a map node that fans out over a slice
// input. itemInput is the wire name inside the sub-graph that will receive
// each element. The input slice wire must already be declared via Input().
// Returns a MapConfigBuilder to define the sub-graph vertices; call
// CollectInto on it to finalize and return here.
func (vb *VertexBuilder) MapOver(itemInput string) *MapConfigBuilder {
	if itemInput == "" {
		vb.err = errors.Join(vb.err, fmt.Errorf("vertex %q: MapOver itemInput cannot be empty", vb.name))
	}
	return &MapConfigBuilder{
		vb:          vb,
		itemInput:   itemInput,
		subVertices: make(map[string]*config.VertexConfig),
	}
}

// Done returns to the Graph builder.
func (vb *VertexBuilder) Done() *Builder {
	return vb.builder
}

// Vertex returns to the Graph builder and then enters or returns a vertex builder.
func (vb *VertexBuilder) Vertex(name string) *VertexBuilder {
	return vb.builder.Vertex(name)
}

// Build constructs the Graph from the Graph builder.
func (vb *VertexBuilder) Build() (*Graph, error) {
	return vb.builder.Build()
}
