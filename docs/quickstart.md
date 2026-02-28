## Quick Start

### 1. 安装与环境准备

- 要求：Go 1.20+（建议 1.21+）
- 安装依赖：

```bash
git clone <your-repo-url> sath
cd sath
go test ./...
```

- 配置 OpenAI Key：

```bash
# Windows PowerShell
$env:OPENAI_API_KEY="your-key"

# Linux / macOS
export OPENAI_API_KEY="your-key"
```

### 2. 运行对话 Agent Demo

```bash
cd cmd/demo
go run .
```

在终端输入问题，得到回复，输入 `exit` 退出。

### 3. 运行工具调用 Demo（Function Calling）

```bash
cd cmd/tool_demo
go run .
```

该 Demo 展示了模型通过 tools API 调用本地 `calculator_add` 工具完成加法计算。

### 4. 在代码中创建一个 Agent

示例：使用 `OpenAIClient + ChatAgent + BufferMemory`：

```go
cfg := config.FromEnv()
m, _ := model.NewOpenAIClient()
mem := memory.NewBufferMemory(cfg.MaxHistory)
core := agent.NewChatAgent(m, mem)

handler := middleware.Chain(
	func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		return core.Run(ctx, req)
	},
	middleware.RecoveryMiddleware,
	middleware.LoggingMiddleware,
	middleware.MetricsMiddleware,
	middleware.TracingMiddleware,
)
```

### 5. 从配置文件加载

创建 `config.yaml`：

```yaml
model: openai/gpt-4o
max_history: 5
middlewares:
  - logging
  - metrics
  - tracing
```

在代码中加载：

```go
cfg, err := config.Load("config.yaml")
_ = cfg
```

### 6. CLI 与开发者体验

项目提供统一 CLI `sath`，便于初始化项目、跑示例和启动 HTTP 服务：

```bash
go build -o sath ./cmd/sath
./sath --help
```

| 命令 | 说明 |
|------|------|
| `sath init` | 在当前目录（或 `-d <dir>`）生成 `main.go` 与 `config.yaml` 骨架 |
| `sath demo` | 运行内置对话 REPL（等价于 `cmd/demo`，需设置 `OPENAI_API_KEY`） |
| `sath serve` | 启动 HTTP 服务：`POST /chat` 对话，`GET /metrics` 指标；可用 `-a :8080`、`-c config.yaml` |

新项目可先执行 `sath init`，再执行 `go mod init <module>`、`go get github.com/sath` 与 `go run .`。

