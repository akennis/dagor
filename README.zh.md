[英文](README.md) | 中文

# Dagor

Dagor是一个高性能DAG算子执行框架，专为高并发在线服务设计。它将复杂的业务逻辑解耦为独立的算子，通过DAG进行灵活编排，自动处理并行调度与数据注入。适用于搜索、推荐、广告、实时特征计算等工业级场景。

## ✨ 核心亮点

- **字段依赖**：框架自动推导顶点依赖关系，用户只需声明输入/输出字段
- **零手动注入**：框架自动完成 Input/Output 字段映射与数据传递
- **配置驱动**：通过JSON定义复杂的业务流程，实现业务拓扑与代码逻辑的完全解耦
- **极致性能**：协程池异步调度 + 算子池化 + 依赖拓扑优化，最大化并行度，极致降低 GC
- **极简 API**：简洁干净的JSON语法。开箱即用的API接口，用户只需关注核心业务逻辑
- **代码生成**：自动生成算子辅助代码，减少代码编写

## 🧩 核心概念

概念

- **Operator (算子)**：独立的计算执行单元。包含具体的业务逻辑。
- **Vertex (顶点)**：图中的顶点。每个 顶点 对应一个 Operator (算子)。
- **Edge（边）**：顶点间的依赖关系。一条边对应顶点的一个输出数据（变量）。
- **Graph (图)**：有多个顶点和边组成的DAG。一个图对应完整的业务执行流程。
- **Engine (引擎)**：Graph运行时容器，用来执行图。具体负责协程调度、状态存储及变量注入。

Graph、Vertex、Operator关系示意图

![dag](/docs/images/dag.png)

## 📦 安装

```Bash
go get github.com/wwz16/dagor
```

## 🚀 快速开始

下面通过一个极简数学计算示例，演示完整流程。完整例子见 [examples/math/](/examples/math/)

### 1.定义算子(Operator)

AddOp 示例，利用 dag 标签声明输入输出，框架会自动完成数据的“地址绑定”。

```Go
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

约定

- 使用标签`dag:"input"`声明输入字段，`dag:"output"`声明输出字段
- 输入字段必须为**指针类型**（*int、*string 等），便于框架高效传递
- 输入字段**只读**，禁止修改，保证并发安全

### 2.配置DAG图(Graph Configuration)

准备JSON配置

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

可视化流程图

![math demo](/docs/images/demo.png)

约定

- vertex名字需**全局唯一**
- vertex outputs 字段名需**全局唯一**

### 3.构图运行

```Go
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

## 🛠 进阶特性

### 代码生成

手动实现 IOperator 的所有接口非常繁琐。为减少重复编写，daggen支持自动生成算子辅助代码。参考例子[examples/math/op/](/examples/math/op/)

1.在算子文件中添加指令

```Go
//go:generate daggen -type=AddOp -output=add_op_gen.go
//go:generate daggen -type=ConstOp -output=const_op_gen.go
//go:generate daggen -type=LogOp -output=log_op_gen.go
```

2.执行生成

```bash
go generate ./...
```

### 动态参数解析 (Params)

算子可以通过`Params`直接读取参数，支持路径访问，无需提前定义结构体。

```Go
func (op *MyOp) Setup(params *config.Params) error {
    // Support "a.b.c" and "tags.0"
    op.threshold = params.GetFloat64("config.nodes.0.threshold", 0.5)
    return nil
}
```

### 可视化 (Visualization)

利用 dagviz 工具，你可以将复杂的 JSON 配置一键转为直观的拓扑图，方便架构评审与排错。

```bash
python dagviz.py -i demo.json -o demo_tb.png
```

## 📄 许可证 (License)

根据 [MIT许可证](/LICENSE) 分发。
