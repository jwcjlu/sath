## API Reference（简要）

本节只列出最常用的核心类型与构造函数，详细请参阅源码注释。

### 1. model

- `type Message struct { Role, Content string; Metadata map[string]any }`
- `type Generation struct { Text string; Raw any }`
- `type Embedding struct { Vector []float32; Raw any }`
- `type Model interface {`
  - `Generate(ctx context.Context, prompt string, opts ...Option) (*Generation, error)`
  - `Chat(ctx context.Context, messages []Message, opts ...Option) (*Generation, error)`
  - `Embed(ctx context.Context, texts []string, opts ...Option) ([]Embedding, error)`
  - `}`
- OpenAI：
  - `func NewOpenAIClient() (*OpenAIClient, error)`
  - `func (c *OpenAIClient) ChatWithTools(ctx context.Context, messages []Message, reg *tool.Registry, opts ...Option) (*Generation, error)`
- 多模型：
  - `func NewFromIdentifier(id string) (Model, error)` // e.g. `"openai/gpt-4o"`, `"dashscope/qwen-turbo"`, `"ollama/llama3"`
  - `func NewMultiModelFromMap(...)`、`func NewMultiModelWithOptions(opts MultiModelOptions) (*MultiModel, error)`；策略：`StrategyByName`、`StrategyByCost`、`StrategyByLatency`
- `Generation.TokenUsage *TokenUsage`（InputTokens、OutputTokens，供指标上报）

### 2. agent

- `type Request struct { Messages []model.Message; Metadata map[string]any; RequestID string }`
- `type Response struct { Text string; Metadata map[string]any }`
- `type Agent interface { Run(ctx context.Context, req *Request) (*Response, error) }`
- Chat：
  - `func NewChatAgent(m model.Model, mem memory.Memory, opts ...Option) *ChatAgent`
  - `WithEventBus(bus *events.Bus)`：在生命周期关键点发布事件
- ReAct：
  - `func NewReActAgent(m model.Model, mem memory.Memory, tools *tool.Registry, opts ...ReActOption) *ReActAgent`
- Plan-and-Execute：
  - `func NewPlanExecuteAgent(planner model.Model, worker Agent) *PlanExecuteAgent`

### 3. memory

- 短期：
  - `type Memory interface { Add(ctx, Entry) error; GetRecent(ctx, n int) ([]Entry, error); Clear(ctx) error }`
  - `func NewBufferMemory(max int) *BufferMemory`
- 向量：
  - `type VectorStore interface { Add(ctx context.Context, entry VectorEntry) error; Search(ctx context.Context, query []float32, k int) ([]VectorEntry, error); Clear(ctx context.Context) error }`
  - `func NewInMemoryVectorStore() *InMemoryVectorStore`
  - `func EmbedAndAdd(ctx context.Context, m model.Model, store VectorStore, id, text string, meta map[string]any) error`
- 摘要：
  - `func NewSummaryMemory(m model.Model, maxItems int) *SummaryMemory`
  - `func (s *SummaryMemory) SummarizeAndAdd(ctx context.Context, id string, messages []model.Message) (SummaryEntry, error)`
- 管理器：
  - `func NewManager(m model.Model, store VectorStore, summary *SummaryMemory, cfg ManagerConfig) *Manager`

### 4. parser

- `type Parser interface { Parse(text string, v any) error }`
- `JSONParser`、`KVParser`、`TableParser`（表格 → `[][]string` 或 `[]map[string]string`）
- `func NewTableParser() *TableParser`

### 5. tool

- `type Tool struct { Name, Description string; Parameters map[string]any; Execute ExecuteFunc }`
- `func NewRegistry() *Registry`
- `func (r *Registry) Register(t Tool) error`
- 内置示例工具：
  - `func RegisterCalculatorTool(r *Registry) error`

### 6. middleware

- 核心：
  - `type Handler func(ctx context.Context, req *agent.Request) (*agent.Response, error)`
  - `type Middleware func(Handler) Handler`
  - `func Chain(final Handler, mws ...Middleware) Handler`
  - `type OrderedMiddleware struct { Order int; Mw Middleware }`、`func ChainBuilder(final Handler, ordered ...OrderedMiddleware) Handler`
  - `func MergeGlobalLocal(final Handler, global, local []Middleware) Handler`
- 常用中间件构造：
  - `LoggingMiddleware`
  - `RecoveryMiddleware`
  - `DebugMiddleware(enabled bool)`：调试模式下输出脱敏请求/响应摘要与错误栈（F5.5）
  - `CacheMiddleware(store *CacheStore)`
  - `RateLimitMiddleware(limiter *RateLimiter, keyFn KeyFunc)`
  - `ContentSafetyMiddleware(filter ContentFilter)`
  - `MetricsMiddleware`
  - `TracingMiddleware`

### 7. config

- `type Config struct { ModelName string; MaxHistory int; Middlewares []string }`
- `func Load(path string) (Config, error)` // YAML/JSON
- `func LoadWithEnv(path string) (Config, error)` // Load + 环境变量覆盖
- `func LoadForEnv(env, dir string) (Config, error)` // 多环境 config.<env>.yaml
- `func ApplyEnvOverrides(cfg *Config)` // 用环境变量覆盖 cfg
- `func FromEnv() Config` // ENV: OPENAI_MODEL, AGENT_MAX_HISTORY

### 8. templates

- `func NewChatAgentHandler(m model.Model, mem memory.Memory, mws ...middleware.Middleware) middleware.Handler`
- `func NewChatAgentHandlerFromConfig(cfg config.Config, middlewareByName map[string]middleware.Middleware) (middleware.Handler, error)`
- `func DefaultMiddlewareMap() map[string]middleware.Middleware`
- `type RAGConfig struct { TopK int }`
- `func NewRAGHandler(m model.Model, store memory.VectorStore, cfg RAGConfig, mws ...middleware.Middleware) middleware.Handler`

### 9. obs（可观察性）

- 指标：`MetricsHandler() http.Handler`、`ObserveAgentRequest(agentName, status, duration)`
- 健康检查（F5.4）：`HealthCheckFunc func(ctx context.Context) error`、`HealthHandler(checks map[string]HealthCheckFunc) http.Handler`；返回 JSON：`status`（ok/unhealthy）、`checks`（组件名→状态）
- 调试（F5.5）：见 middleware.DebugMiddleware

### 10. events（生命周期事件）

- `type Kind string`：`AgentInit`、`RunStarted`、`ModelResponded`、`ToolExecuted`、`RunCompleted`、`RunError`
- `type Event struct { Kind, Payload map[string]any, RequestID string, At time.Time }`
- `type Listener func(ctx context.Context, e Event)`
- `func NewBus() *Bus`
- `func (b *Bus) Subscribe(async bool, l Listener)` / `Publish(ctx, e Event)`
- `func DefaultBus() *Bus` / `SetDefaultBus(b *Bus)`

### 11. plugin（插件注册中心）

- `RegisterModelProvider(provider string, f model.ModelProvider)`：扩展 `NewFromIdentifier`
- `RegisterTool(t tool.Tool)` / `RegisteredTools() []tool.Tool`
- `RegisterMiddleware(name string, mw middleware.Middleware)` / `RegisteredMiddlewares() map[string]middleware.Middleware`
- `RegisterEventListener(async bool, l events.Listener)` / `ApplyListenersTo(bus *events.Bus)`

详见 [Extending](extending.md)。

