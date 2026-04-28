# Temperature Pipeline Example

This example demonstrates two key dagor framework concepts working together:

1. **Fluent builder API** — the entire graph is declared in a single chained expression without any JSON configuration file.
2. **Multi-step subgraph mapping** — a `MapOver` vertex fans out over a slice and runs each element through a *pipeline of two operators* before collecting the results.

## What it does

A source operator emits a list of Celsius temperatures. A map vertex processes every element concurrently through a two-step subgraph:

| Step | Operator | Input | Output |
|------|----------|-------|--------|
| 1 | `ToFahrenheitOp` | `°C` | `°F` |
| 2 | `CategorizeOp` | `°F` | label (`freezing` / `cold` / `warm` / `hot`) |

The collected labels are written to the `categories` output wire.

## DAG

```
Outer graph
───────────────────────────────────────────────────────────
 ┌──────────────────┐         ┌──────────────────────────┐
 │  source          │         │  classify_all  [MAP]      │
 │  (TempSourceOp)  │         │                           │
 │                  │celsius  │  ┌─────────────────────┐  │
 │  Temps ──────────┼─────────┼─►│ Subgraph (per item) │  │
 └──────────────────┘  _temps │  │                     │  │
                              │  │  item (*float64)    │  │
                              │  │    │                │  │
                              │  │    ▼                │  │
                              │  │  to_fahrenheit      │  │
                              │  │  (ToFahrenheitOp)   │  │
                              │  │    │ fahrenheit      │  │
                              │  │    ▼                │  │
                              │  │  categorize         │  │
                              │  │  (CategorizeOp)     │  │
                              │  │    │ category        │  │
                              │  └────┼────────────────┘  │
                              │       │ collect            │
                              └───────┼────────────────────┘
                                      │
                                      ▼
                               categories (*[]any)
```

**Wire flow:**
- `celsius_temps` — `*[]float64` from `source` → input slice for the map vertex
- `item` — `*float64` injected per element into the subgraph (external wire)
- `fahrenheit` — `*float64` connecting `to_fahrenheit` → `categorize` inside the subgraph
- `category` — `string` result collected from each subgraph run
- `categories` — `*[]any` of strings written to the parent graph by the map vertex

## Framework concepts illustrated

### Fluent builder with `MapOver`

The graph is built entirely in code using the chainable builder API. The `MapOver` call switches into subgraph-definition mode; `SubVertex` adds operators to that subgraph; `CollectInto` finalises it and returns to the outer builder.

```go
graph.NewBuilder("temperature_pipeline").
    Vertex("source").
        Op("TempSourceOp").
        Params(map[string]any{"temps": input}).
        Output("Temps", "celsius_temps").
    Vertex("classify_all").
        Input("Items", "celsius_temps").
        MapOver("item").              // ← enter subgraph scope
            SubVertex("to_fahrenheit").
                Op("ToFahrenheitOp").
                Input("Celsius", "item").
                Output("Fahrenheit", "fahrenheit").
            SubVertex("categorize").  // ← second op in the subgraph pipeline
                Op("CategorizeOp").
                Input("Temp", "fahrenheit").
                Output("Category", "category").
            CollectInto("category", "categories"). // ← exit subgraph scope
    Build()
```

### Multi-step subgraph

Unlike the [map example](../map/) which uses a single operator per element, the subgraph here chains two operators. The `fahrenheit` wire connects them: `to_fahrenheit` produces it and `categorize` consumes it. The engine builds and executes a full DAG for each element, resolving this internal dependency automatically.

### Pointer-based wire convention

Each element of the input `[]float64` slice is wrapped in a `*float64` pointer by the engine before being injected as the `item` wire. Operators that consume a wire declare their input field as a pointer (`*float64`) and receive it via `SetInputField`. Output fields are also stored by pointer (`&op.Fahrenheit`), so downstream operators read the value written by the previous step.

## Running

```bash
cd examples/temperature
go run .
```

Expected output:

```
input (°C) → category
  -10°C → freezing
  0°C → cold
  20°C → warm
  37°C → hot
  100°C → hot
```
