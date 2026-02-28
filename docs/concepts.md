## 核心概念一览

### 1. Model

- 抽象统一的模型接口：
  - `Generate(ctx, prompt)`：单轮生成。
  - `Chat(ctx, []Message)`：多轮对话。
  - `Embed(ctx, []string)`：文本向量化。
- 当前内置：基于 `go-openai` 的 `OpenAIClient`。

### 2. Agent

- 接口：`Run(ctx, *Request) (*Response, error)`。
- 内置实现：
  - `ChatAgent`：简单对话 Agent。
  - `ReActAgent`：基于 tools API 的 ReAct 循环 Agent。
  - `PlanExecuteAgent`：规划 + 执行（Plan-and-Execute）模式。

### 3. Memory

- 短期记忆：`BufferMemory`，存最近 N 条对话。
- 向量记忆：`VectorStore` + `InMemoryVectorStore`，用于长期检索。
- 摘要记忆：`SummaryMemory`，将长对话压缩为简介。
- 管理器：`memory.Manager` 协调短期 / 向量 / 摘要三类记忆。

### 4. Tools & Function Calling

- `tool.Tool`：
  - `Name`、`Description`、`Parameters`（JSON Schema 风格）、`Execute(ctx, params)`.
- `tool.Registry`：注册 / 查找工具。
- `OpenAIClient.ChatWithTools`：
  - 使用 OpenAI tools API 和本地 `Registry` 实现一次工具调用步骤。

### 5. Middleware

- 抽象：`type Handler func(ctx, *agent.Request) (*agent.Response, error)` + `type Middleware func(Handler) Handler`。
- 通用中间件：
  - 日志：`LoggingMiddleware`
  - 恢复：`RecoveryMiddleware`
  - 缓存：`CacheMiddleware`
  - 限流：`RateLimitMiddleware`
  - 内容安全：`ContentSafetyMiddleware`
  - 指标：`MetricsMiddleware`
  - 追踪：`TracingMiddleware`

### 6. Observability

- 日志：`obs.Logger`（包装标准库 log）。
- 指标：`obs.ObserveAgentRequest` + `obs.MetricsHandler`（Prometheus）。
- 追踪：`obs.InitTracer`（OpenTelemetry stdout exporter）。

### 7. 配置与模板

- 配置：`config.Config` + `config.Load`（YAML/JSON） + `config.FromEnv`。
- 模板：
  - `templates.NewChatAgentHandler(...)`：对话 Agent 模板。
  - `templates.NewRAGHandler(...)`：文档问答（RAG）模板。

### 8. 插件与事件（扩展）

- **插件**：`plugin` 包提供注册中心，插件通过 `init()` 注册模型 Provider、工具、中间件与事件监听器；详见 [Extending](extending.md)。
- **事件**：`events.Bus` 在 Agent 生命周期关键点（RunStarted、ModelResponded、RunCompleted、RunError 等）发布事件，支持同步/异步监听，用于日志、审计、指标等。

