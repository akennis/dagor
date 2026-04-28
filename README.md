English | [中文](README.zh.md)

# Dagor

Dagor is a high-performance DAG (Directed Acyclic Graph) operator execution framework designed for high-concurrency online services. It decouples complex business logic into independent operators, enabling flexible orchestration via DAGs with automated parallel scheduling and data injection.

It is ideal for industrial-grade scenarios such as search engines, recommendation systems, advertising platforms, and real-time feature engineering.

## ✨ Key Highlights

* **Field-Level Dependency**: The framework automatically deduces vertex dependencies; users only need to declare input/output fields.
* **Zero-Code Injection**: Automated mapping of input/output fields and seamless data transmission between operators.
* **Configuration-Driven**: Define complex business workflows via JSON, achieving complete decoupling of business topology from code logic.
* **Extreme Performance**: Features a goroutine pool for asynchronous scheduling, operator pooling, and topology optimization to maximize parallelism and minimize GC pressure.
* **Conditional Branching**: Skip vertices at runtime via named predicates, with transitive propagation and coalesce merge for mutually-exclusive branches.
* **Map Nodes**: Fan out a sub-graph over a dynamically-produced slice, processing every element concurrently and collecting results — no list required at graph-build time.
* **Developer-Friendly API**: Clean JSON syntax and a fluent builder API allow developers to focus purely on core business logic.
* **Code Generation**: Automated generation of operator code to reduce manual development effort.

## 🧩 Core Concepts

* **Operator**: The independent unit of computation containing specific business logic.
* **Vertex**: A node in the graph. Each vertex corresponds to a specific Operator instance.
* **Edge**: Represents a dependency between vertices, corresponding to an output data field (variable) from one vertex.
* **Graph**: A DAG composed of multiple vertices and edges, representing a complete business workflow.
* **Engine**: The runtime container for the Graph. It handles goroutine scheduling, state management, and variable injection.

Relationship between **Graph**、**Vertex** and **Operator**:
![dag](/docs/images/dag.png)

## 📦 Installation

```bash
go get github.com/wwz16/dagor
```

## 🚀 Quick Start

Below is a minimalist mathematical calculation example. For the full example, see [examples/math/](/examples/math/).

### 1. Define an Operator

Take `AddOp` as an example. Use the `dag` tag to declare inputs and outputs; the framework will automatically handle data binding.

```go
import (
    "context"
    "fmt"
    "log"

    "github.com/wwz16/dagor/config"
    "github.com/wwz16/dagor/operator"
)

type AddOp struct {
    a   *int `dag:"input"`
    b   *int `dag:"input"`
    sum int  `dag:"output"`
}

// Setup parses and validates params and setup internal fields.
func (op *AddOp) Setup(params *config.Params) error {
    return nil
}

// Run executes the operator.
func (op *AddOp) Run(ctx context.Context) error {
    if op.a == nil || op.b == nil {
        return fmt.Errorf("AddOp: missing required input 'a' or 'b'")
    }
    op.sum = *op.a + *op.b
    return nil
}

// Reset resets the operator state and clear internal fields in order to reuse next time.
func (op *AddOp) Reset() error {
    return nil
}

func init() {
    // register operator
    if err := operator.RegisterOp[AddOp](); err != nil {
        log.Fatalf("RegisterOp[AddOp] error: %v", err)
    }
}
```

**Conventions:**

* Use `dag:"input"` for input fields and `dag:"output"` for output fields.
* Input fields must be **pointer types** (`*int`, `*string`, etc.) for high-efficiency transmission.
* Input fields are **read-only** to ensure concurrency safety.

### 2. Configure the Graph

Prepare a JSON configuration to define the topology.

```json
{
  "name": "math_demo", // graph name
  "vertices": { // all vertices
    "const10": { // vertex name
      "op": "ConstOp", // operator class name
      "params": { // operator parameters
        "in": 10
      },
      "outputs": {  // output data
        "out": "n1"  // `out` is operator field name that defined in operator class, `n1` is vertex field name that used for graph dependencies
      }
    },
    "const20": {
      "op": "ConstOp",
      "params": {
        "in": 20
      },
      "outputs": {
        "out": "n2"
      }
    },
    "add": {
      "op": "AddOp",
      "inputs": {
        "a": "n1",
        "b": "n2"
      },
      "outputs": {
        "result": "n3"
      }
    },
    "log": {
      "op": "LogOp",
      "params": {
        "base": 10
      },
      "inputs": {
        "x": "n3"
      },
      "outputs": {
        "result": "answer"
      }
    }
  }
}
```

Visualize the dag:

![math demo](/docs/images/demo.png)

**Conventions:**

* vertex name must be **globally unique**
* vertex output field name must be **globally unique**

### 3. Run the Engine

```go
import (
    "log"
    "fmt"

    "github.com/wwz16/dagor"
    "github.com/panjf2000/ants/v2"
)

func main() {
    // 1. Init global goroutine pool. 
    // Take ants as an example, you may change to other pools.
    p, err := ants.NewPool(3)
    if err != nil {
        log.Printf("ants.NewPool error %v\n", err)
        return
    }
    defer p.Release()

    // 2. Build graph.
    conf := `{
      "name": "math_demo",
      ...
    }`
    g, err := dagor.NewGraphFromJson(conf)
    if err != nil {
        log.Printf("NewGraphFromJson error %v\n", err)
        return
    }

    // 3. Run graph engine
    // Init context.
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    // Create engine instance
    eng, err := dagor.NewEngine(g, p)
    if err != nil {
        log.Printf("NewEngine error %v\n", err)
        return
    }
    defer eng.Close(ctx)

    // Run the graph.
    if err = eng.Run(ctx); err != nil {
        log.Printf("Run error %v\n", err)
        return
    }

    // 4. Get the output data.
    v, ok := eng.GetOutput("answer")
    if !ok {
        log.Printf("GetOutput error\n")
        return
    }
    res := *v.(*float64)
    log.Printf("result: %f\n", res)
}
```

## 🛠 Advanced Features

### Conditional Vertices

A vertex can be made conditional by setting the `condition` field to the name of a registered **predicate**. If the predicate returns `false` at runtime the vertex (and all vertices that depend solely on its outputs) is skipped.

**1. Register a predicate** – predicates receive the current graph's output map and return a boolean.

```go
import "github.com/wwz16/dagor/predicate"

predicate.Register("positive", func(inputs map[string]any) bool {
    ptr, ok := inputs["source_out"].(*int)
    if !ok || ptr == nil {
        return false
    }
    return *ptr > 0
})
```

**2. Reference it in the vertex config:**

```json
{
  "filter": {
    "op": "FilterOp",
    "condition": "positive",
    "inputs":  { "in": "source_out" },
    "outputs": { "out": "filter_out" }
  }
}
```

Or in Go config:

```go
"filter": {
    Op:        "FilterOp",
    Condition: "positive",
    Inputs:    map[string]string{"in": "source_out"},
    Outputs:   map[string]string{"out": "filter_out"},
},
```

**3. Check whether a vertex was skipped after `eng.Run`:**

```go
if eng.VertexSkipped("filter") {
    log.Println("filter was skipped")
}
```

See the full example in [examples/conditional/](/examples/conditional/).

---

### Coalescing Mutually-Exclusive Branches

When two branches are guarded by complementary predicates (exactly one always runs), use a **coalesce vertex** to merge their outputs into a single value. Without `merge: "coalesce"` the engine would propagate the skip from the branch that did not run and refuse to execute the output node.

```
source ──► det_branch  (condition=positive)     ──► coalesce (merge=coalesce)
       └─► ai_branch   (condition=not_positive) ──►
```

**Config (JSON):**

```json
{
  "coalesce": {
    "op":     "CoalesceIntOp",
    "merge":  "coalesce",
    "inputs":  { "A": "det_out", "B": "ai_out" },
    "outputs": { "Result": "coalesced_out" }
  }
}
```

**Config (Go):**

```go
"coalesce": {
    Op:      "CoalesceIntOp",
    Merge:   config.MergeCoalesce,
    Inputs:  map[string]string{"A": "det_out", "B": "ai_out"},
    Outputs: map[string]string{"Result": "coalesced_out"},
},
```

Import the built-in operators package to make the coalesce operators available:

```go
import _ "github.com/wwz16/dagor/operator/builtin"
```

**Built-in coalesce operators (2-input):**

| Operator name       | Type      |
|---------------------|-----------|
| `CoalesceStringOp`  | `string`  |
| `CoalesceIntOp`     | `int`     |
| `CoalesceFloat64Op` | `float64` |
| `CoalesceBoolOp`    | `bool`    |

**N-input variants** (configure arity via `params.n`):

| Operator name        | Type      |
|----------------------|-----------|
| `CoalesceNStringOp`  | `string`  |
| `CoalesceNIntOp`     | `int`     |
| `CoalesceNFloat64Op` | `float64` |
| `CoalesceNBoolOp`    | `bool`    |

```json
{
  "coalesce": {
    "op":     "CoalesceNIntOp",
    "merge":  "coalesce",
    "params": { "n": 3 },
    "inputs":  { "Input0": "branch0_out", "Input1": "branch1_out", "Input2": "branch2_out" },
    "outputs": { "Result": "final_out" }
  }
}
```

`CoalesceOp` returns the first non-nil input in declaration order (`A` before `B`, `Input0` before `Input1`, …). It errors if every branch was skipped (all inputs are nil).

---

### Map Nodes

A **map node** fans out execution of a sub-graph over every element of a slice input concurrently, then collects the per-element results into a `[]any` output wire. This is the idiomatic way to apply a multi-step pipeline to a list that materialises as the output of an earlier node.

```
source ──► [map node] ──► []any results
               │
               └─ sub-graph (runs once per element, in parallel)
                     step1 ──► step2 ──► … ──► result wire
```

#### How it works

1. The map node reads a slice from its single input wire.
2. Each element is wrapped in a pointer (`*T`) and injected as the **item wire** inside the sub-graph, consistent with dagor's pointer-based wire convention.
3. One sub-graph execution is submitted to the goroutine pool per element; all run concurrently.
4. The value of the **result wire** from each execution is dereferenced and appended to a `[]any` output slice.
5. Downstream vertices receive `[]any` and type-assert to the expected concrete type.

#### Sub-graph operator convention

Sub-graph operators that consume the item wire declare their input as `*T` and type-assert in `SetInputField`:

```go
type DoubleOp struct {
    In  *int `dag:"input"`
    Out int  `dag:"output"`
}

func (op *DoubleOp) SetInputField(field string, value any) error {
    if field == "In" {
        v, ok := value.(*int)
        if !ok {
            return fmt.Errorf("expected *int, got %T", value)
        }
        op.In = v
    }
    return nil
}
```

#### JSON configuration

```json
{
  "source": {
    "op": "SourceOp",
    "params": { "values": [1, 2, 3, 4, 5] },
    "outputs": { "Items": "raw_items" }
  },
  "double_all": {
    "inputs":  { "Items": "raw_items" },
    "outputs": { "Results": "doubled_items" },
    "map": {
      "item_input":    "item",
      "result_output": "result",
      "subgraph": {
        "external_wires": ["item"],
        "vertices": {
          "double": {
            "op":      "DoubleOp",
            "inputs":  { "In": "item" },
            "outputs": { "Out": "result" }
          }
        }
      }
    }
  }
}
```

**Key fields:**

| Field | Description |
|---|---|
| `map.item_input` | Wire name inside the sub-graph that receives each element (`*T`). |
| `map.result_output` | Wire name inside the sub-graph whose value is collected per element. |
| `map.subgraph.external_wires` | Must list the item wire — tells the sub-graph it has no producer vertex for this wire. |

#### Fluent Builder API

```go
g, err := graph.NewBuilder("map_demo").
    Vertex("source").
        Op("SourceOp").
        Params(map[string]any{"values": []int{1, 2, 3, 4, 5}}).
        Output("Items", "raw_items").
    Vertex("double_all").
        Input("Items", "raw_items").
        MapOver("item").                  // declares the item wire name
            SubVertex("double").
                Op("DoubleOp").
                Input("In", "item").
                Output("Out", "result").
            CollectInto("result", "doubled_items"). // result wire → output wire
    Build()
```

`MapOver(itemInput)` returns a `MapConfigBuilder`. Chain `SubVertex` calls to define the sub-graph, then terminate with `CollectInto(resultOutput, outputWire)` which returns to the parent `VertexBuilder` so the fluent chain continues normally.

#### Reading the output

```go
out, _ := eng.GetOutput("doubled_items")
results := out.([]any)
for _, v := range results {
    fmt.Println(v.(int)) // type-assert to the concrete element type
}
```

See the full example in [examples/map/](/examples/map/).

---

### Automated Code Generation

Implementing every method of the `IOperator` interface can be repetitive. `daggen` automates this process.

1.**Add directives to your operator file:**

```go
//go:generate daggen -type=AddOp -output=add_op_gen.go
//go:generate daggen -type=ConstOp -output=const_op_gen.go
//go:generate daggen -type=LogOp -output=log_op_gen.go
```

2.**Run generation:**

```bash
go generate ./...
```

### Fluent Builder API

As an alternative to JSON configuration, you can use the fluent builder API provided by the `graph` package to construct DAGs programmatically. This is particularly useful for dynamic graph construction or when you prefer type safety.

```go
import (
    "github.com/wwz16/dagor/graph"
)

func buildGraph() (*graph.Graph, error) {
    return graph.NewBuilder("math_demo").
        Vertex("const10").
            Op("ConstOp").
            Params(map[string]int{"in": 10}).
            Output("out", "n1").
        Done().
        Vertex("const20").
            Op("ConstOp").
            Params(map[string]int{"in": 20}).
            Output("out", "n2").
        Done().
        Vertex("add").
            Op("AddOp").
            Input("a", "n1").
            Input("b", "n2").
            Output("sum", "n3").
        Done().
        Vertex("log").
            Op("LogOp").
            Params(map[string]int{"base": 10}).
            Input("x", "n3").
            Output("result", "answer").
        Done().
        Build()
}
```

### Dynamic Parameter Parsings

Operators can read parameters directly using `Params`, which supports path-based access without pre-defining structures.

```go
func (op *MyOp) Setup(params *config.Params) error {
    // Supports nested path access like "a.b.c" or "array.0"
    op.threshold = params.GetFloat64("config.nodes.0.threshold", 0.5)
    return nil
}
```

### Visualization

Use the `dagviz` tool to convert complex JSON configurations into intuitive topological diagrams for easier review and debugging.

```bash
python dagviz.py -i demo.json -o workflow.png
```

## 📄 License

Distributed under the [MIT License](/LICENSE).
