# 对话式数据查询与修改系统 — 开发任务拆解（Cursor Agent 版）

本文档将《基于对话的数据查询与修改系统-完整需求文档》拆解为可在 **Cursor Agent** 中逐条执行的开发任务，并与当前 **sath** 框架的架构与风格保持一致。

---

## 一、与当前框架的对应关系

| 需求域         | 框架内对应/扩展方式 |
|----------------|----------------------|
| 对话与多轮     | 沿用 `agent.Agent`、`memory.Memory`（如 BufferMemory 存会话），新增「数据对话」专用 Agent 或模板。 |
| 意图/实体      | 新包 `intent` 或 `nl2dsl`，可依赖 `model.Model` 做 LLM 调用，输出结构化意图与实体。 |
| 元数据         | 新包 `metadata`，按数据源类型拉取并缓存，存储可复用 `memory` 或独立存储接口。 |
| DSL 生成与校验 | 新包 `dsl`（或合入 `nl2dsl`），输入意图+实体+元数据，输出可执行 DSL；校验可调用各数据源驱动。 |
| 数据源与执行   | 新包 `datasource`（驱动接口、连接池、健康检查）+ `executor`（执行 DSL、超时、只读/行数限制）。 |
| 权限与审计     | 沿用 `middleware` 链 + `errs`；新增权限检查中间件或 executor 内校验；审计用 `events.Bus` 或专用 logger。 |
| 结果呈现       | 新包 `result`（格式化、分页、导出占位）或直接在 handler 内组装；图表可先由前端负责，后端只提供结构化数据。 |
| HTTP/API       | 沿用 `cli/serve` 的 Handler 模式与 `templates`；新增 `templates.NewDataQueryHandler` 或单独 `cmd/dataquery`。 |

**风格与约定**：Option 模式配置、接口+实现分离、错误用 `errs`、可观测用 `obs` 与 `events`；新包放在仓库根目录下与 `agent`、`model` 同级。

---

## 二、任务总览与依赖顺序

```
P0 基础设施
  T-01 数据源驱动接口与连接配置
  T-02 MySQL 驱动实现（连接池 + 健康检查）
  T-03 元数据模型与存储接口
  T-04 MySQL 元数据拉取与缓存
  T-05 执行器接口与 MySQL 执行实现（超时、行数限制）

P1 意图与 DSL
  T-06 意图与实体数据结构（意图分类 + 实体抽取结果）
  T-07 意图识别器接口与 LLM 实现（调用 model.Model）
  T-08 NL2SQL 生成（基于意图+实体+元数据，仅 MySQL）
  T-09 DSL 语法/语义校验与执行前确认结构

P2 对话与编排
  T-10 数据对话会话上下文（当前数据源、上轮 DSL、表名等）
  T-11 DataQueryAgent：编排意图识别 → DSL 生成 → 执行 → 结果格式化
  T-12 修改操作二次确认与确认超时

P3 接口与安全
  T-13 数据查询 HTTP API（POST /data/chat，请求/响应格式）
  T-14 数据源配置加载与默认数据源选择
  T-15 权限占位与审计事件（RBAC 占位 + events 发布）

P4 可观测与验收
  T-16 数据查询指标与 trace（意图耗时、DSL 耗时、执行耗时）
  T-17 端到端测试与 MVP 验收用例
```

---

## 三、P0：基础设施

### T-01 数据源驱动接口与连接配置

**目标**：定义与框架风格一致的数据源抽象和连接配置，供后续多种数据库实现复用。

**参考代码**：`model/model.go`（Model 接口）、`config/config.go`（Config 结构）、`plugin/registry.go`（注册扩展）。

**实现要点**：

- 新建包 `datasource`。
- 定义 `DataSource` 接口，至少包含：`ID() string`、`Ping(ctx) error`、`Close() error`；可选 `Execute(ctx, dsl string, opts ExecuteOptions) (Result, error)` 的占位，或本任务只做连接与 Ping。
- 定义 `Config` 结构体：`ID`、`Type`（如 "mysql"）、`DSN` 或 `Host/Port/User/Password/DBName`、`MaxOpenConns`、`MaxIdleConns`、`ConnMaxLifetime`、`ReadOnly`；敏感字段在文档中说明从环境变量或密钥服务读取，不落明文配置。
- 定义 `Registry` 或 `Pool`：`Register(cfg Config) (DataSource, error)`、`Get(id string) (DataSource, error)`、`List() []DataSource`；连接池在实现层（T-02）完成，本任务只定义接口与配置。
- 与现有 `config` 包的关系：可从 `config` 扩展字段（如 `DataSources []datasource.Config`）或独立 YAML 段由 `datasource.LoadFromFile` 读取，保持与 `config.Config` 风格一致。

**验收**：`go build ./datasource/...` 通过；至少一份单元测试构造 `Config` 并调用 `Registry.Register`（可返回“未实现”的 DataSource 占位）。

**依赖**：无。

---

### T-02 MySQL 驱动实现（连接池 + 健康检查）

**目标**：实现 MySQL 类型的 `DataSource`，支持连接池、重试与健康检查，风格与框架一致。

**参考代码**：`model/openai.go`（连接/配置）、`obs/health.go`（HealthCheckFunc）。

**实现要点**：

- 在 `datasource` 包内新增 `mysql.go`（或子包 `datasource/mysql`）。
- 实现 `DataSource` 接口：使用 `database/sql` + `github.com/go-sql-driver/mysql`，连接池参数来自 `datasource.Config`。
- 实现 `Ping(ctx)`：带超时（如 2s）的 `db.PingContext(ctx)`；失败时返回可包装的错误。
- 连接池：`sql.DB` 的 `SetMaxOpenConns`、`SetMaxIdleConns`、`SetConnMaxLifetime` 从 Config 读取；可选实现简单的重试（如 Ping 失败重试 1～2 次）。
- 在 `datasource` 包中提供 `RegisterMySQL(cfg Config) (DataSource, error)` 或通过 `datasource.Register("mysql", factory)` 注册，与 T-01 的 Registry 配合。

**验收**：`go test ./datasource/...` 通过；至少一个集成测试用真实 MySQL 或 testcontainer 进行 Ping（若环境允许）；或使用 mock 的 `*sql.DB` 做单元测试。

**依赖**：T-01。

---

### T-03 元数据模型与存储接口

**目标**：定义与数据库类型无关的元数据模型和存储/缓存接口，便于多数据源扩展。

**参考代码**：`memory/memory.go`（Memory 接口）、`parser/parser.go`（Parser 接口）。

**实现要点**：

- 新建包 `metadata`。
- 定义通用结构（以关系型为例）：`Schema`（如库名）、`Table`（表名、列列表）、`Column`（名、类型、可空、主键等）；可为 Elasticsearch 预留 `Index`、`Mapping` 等类型，首版可实现仅关系型。
- 定义 `Store` 接口：`GetSchema(ctx, datasourceID string) (*Schema, error)`、`Refresh(ctx, datasourceID string) error`、可选 `GetTable(ctx, datasourceID, tableName string) (*Table, error)`。
- 定义内存实现 `InMemoryStore`：map[datasourceID]*Schema；`Refresh` 时由调用方传入拉取函数（见 T-04）。
- 可选：定义缓存策略（TTL、最大条数），在 Store 实现中限制大小，避免内存无限增长。

**验收**：`go build ./metadata/...` 通过；单元测试覆盖 `InMemoryStore` 的 Get/Refresh（Refresh 可接收 mock 拉取函数）。

**依赖**：无（不依赖 T-01/T-02，仅类型与接口）。

---

### T-04 MySQL 元数据拉取与缓存

**目标**：从 MySQL 拉取库/表/列元数据并写入 `metadata.Store`，支持按需刷新与缓存。

**参考代码**：`metadata` 包（T-03）、`datasource` 包（T-02）、`middleware/cache.go`（缓存思路）。

**实现要点**：

- 在 `metadata` 包内新增 `mysql.go` 或子包 `metadata/mysql`。
- 实现 `Fetcher` 接口（若 T-03 未定义则在此定义）：`FetchSchema(ctx, db *sql.DB) (*Schema, error)`，通过 `information_schema` 查询库、表、列、类型、主键等。
- 将 `Store.Refresh(ctx, datasourceID)` 与 `datasource.Registry.Get(datasourceID)` 对接：取得 DataSource 后若为 MySQL 则获取 `*sql.DB`（需在 T-02 中暴露或通过类型断言获取），调用 `Fetcher.FetchSchema`，再写入 `Store`。
- 同步失败时返回错误，由调用方决定是否告警或使用旧缓存；在 Store 实现中可保留“上次成功”的副本并在失败时返回旧数据并带“stale”标记（可选）。

**验收**：单元测试用 mock DB 或内存 SQL 驱动验证拉取结果结构正确；与 T-03 的 Store 集成测试通过。

**依赖**：T-02，T-03。

---

### T-05 执行器接口与 MySQL 执行实现（超时、行数限制）

**目标**：定义执行器接口，实现 MySQL 的 DSL 执行，并支持超时与结果行数/大小限制。

**参考代码**：`datasource`（T-01/T-02）、`middleware/rate_limit.go`（限制思路）、`errs/errs.go`。

**实现要点**：

- 新建包 `executor`。
- 定义 `Executor` 接口：`Execute(ctx, datasourceID, dsl string, opts ExecuteOptions) (*Result, error)`。
- `ExecuteOptions` 包含：`Timeout time.Duration`、`MaxRows int`、`ReadOnly bool`（若为 true 且检测到写操作则拒绝）。
- `Result` 包含：`Rows [][]any` 或 `Columns []string` + `Rows`、`AffectedRows int`、`LastInsertID int64`（可选）、`Error string`（执行错误时的用户向信息）。
- MySQL 实现：通过 `datasource.Registry.Get(datasourceID)` 取得 DataSource，执行 `db.QueryContext(ctx, dsl)` 或 `db.ExecContext`；使用 `ctx` 超时、查询后截断行数至 `MaxRows`；写操作前若 `ReadOnly` 则返回 `errs.ErrBadRequest` 或自定义错误。
- 防注入：本任务内要求调用方传入的 `dsl` 为“已生成且参数化后的语句”；若由上层传入占位符与参数，可在本任务或 T-08 中约定参数化执行方式（如 `db.ExecContext(ctx, query, args...)`）。

**验收**：单元测试用 mock DB 验证超时、MaxRows、ReadOnly 行为；至少一条集成测试执行简单 SELECT（可 mock 数据源）。

**依赖**：T-02，T-01。

---

## 四、P1：意图与 DSL

### T-06 意图与实体数据结构（意图分类 + 实体抽取结果）

**目标**：定义意图类型与实体结构，供意图识别与 NL2DSL 共用，与框架现有类型风格一致。

**参考代码**：`agent/agent.go`（Request/Response）、`model/model.go`（Message）。

**实现要点**：

- 新建包 `intent`（或 `nl2dsl`）。
- 定义意图枚举：如 `IntentQuery`、`IntentInsert`、`IntentUpdate`、`IntentDelete`、`IntentMetadata`、`IntentRewrite`（基于上轮 DSL 的改写）。
- 定义实体结构体：`DatasourceID`、`Schema/Database`、`Table`、`Columns []string`、`Conditions []Condition`（字段、操作符、值）、`Aggregations`（如 Sum/Avg/Count + 字段）、`OrderBy`、`Limit`、`Offset`；修改类增加 `SetClause`、`Values` 等。
- 定义 `ParsedInput` 结构体：`Intent`、`Entities`、`RawNL string`、可选 `PreviousDSL string`（用于 IntentRewrite）；可选 `UncertainFields []string`（需澄清的字段）。
- 不依赖 LLM：本任务仅定义类型与常量，可在测试中手写 `ParsedInput` 用于 T-08。

**验收**：`go build ./intent/...` 通过；单元测试构造 `ParsedInput` 并做简单序列化/反序列化（如 JSON）以验证结构稳定。

**依赖**：无。

---

### T-07 意图识别器接口与 LLM 实现（调用 model.Model）

**目标**：定义意图识别器接口，并实现基于 LLM 的识别（调用 sath 的 model.Model），输出 T-06 的 ParsedInput。

**参考代码**：`agent/agent.go`、`model/openai.go`（Chat）、`parser/json_parser.go`（解析 LLM 输出）。

**实现要点**：

- 在 `intent` 包中定义 `Recognizer` 接口：`Recognize(ctx, sessionID string, messages []model.Message, metadata *metadata.Schema) (*ParsedInput, error)`。
- LLM 实现：构造 prompt（包含当前消息、可选上下文、元数据摘要如表名/列名），调用 `model.Model.Chat(ctx, messages)`；将模型输出解析为 JSON 后映射到 `ParsedInput`（使用 `parser.JSONParser` 或自定义）；若解析失败或意图不明，返回错误或 `UncertainFields` 供上层澄清。
- 与框架一致：接收 `model.Model` 注入（Option 或构造函数参数），不直接依赖具体厂商；错误用 `errs` 或包装。
- 可选：支持“可解释”字段（如 `Reason string`）写入 `ParsedInput`，便于审计或前端展示。

**验收**：单元测试用 fake model（返回固定 JSON）验证解析逻辑；集成测试用真实 Model 跑 1～2 句简单问句（可选）。

**依赖**：T-06；可选依赖 `metadata`（T-03）用于传入表列表等。

---

### T-08 NL2SQL 生成（基于意图+实体+元数据，仅 MySQL）

**目标**：根据 ParsedInput 与元数据生成 MySQL 可执行 SQL（参数化），风格与框架 parser/tool 一致。

**参考代码**：`parser/json_parser.go`、`tool/tool.go`（Tool 描述与执行）、`metadata`（T-03/T-04）。

**实现要点**：

- 新建包 `dsl`（或放在 `intent` 下子包 `dsl`）。
- 定义 `Generator` 接口：`Generate(ctx, input *intent.ParsedInput, meta *metadata.Schema) (dsl string, params []any, err error)`。
- MySQL 实现：根据 `input.Intent` 与 `input.Entities` 拼出 SQL；表名/列名从 `meta` 校验存在性并做标识符转义；条件与值使用占位符 `?`，`params` 为对应参数列表，供 T-05 执行器使用。
- 首版支持：单表 SELECT（含 WHERE、ORDER BY、LIMIT、聚合）；单表 INSERT/UPDATE/DELETE 的简单形式；不支持的语法（如多表 JOIN、子查询）直接返回明确错误。
- 与 T-09 衔接：可返回“待确认”的 DSL 与自然语言描述，供确认环节展示。

**验收**：单元测试覆盖：仅 SELECT、带条件、带 LIMIT、聚合、简单 INSERT/UPDATE/DELETE；错误用例（如表不存在、意图不支持）返回明确错误。

**依赖**：T-06，T-03/T-04。

---

### T-09 DSL 语法/语义校验与执行前确认结构

**目标**：对生成的 DSL 做语法与简单语义校验，并定义执行前确认的请求/响应结构（供修改类操作使用）。

**参考代码**：`errs/errs.go`、`executor`（T-05）、`middleware`（链式处理）。

**实现要点**：

- 在 `dsl` 包中定义 `Validator` 接口：`Validate(ctx, dsl string, meta *metadata.Schema, readOnly bool) error`。
- MySQL 实现：语法校验可调用 MySQL 的 `PREPARE` 或使用简单正则/关键字检查；语义校验可检查表名/列名是否在 `meta` 中、写操作在 `readOnly` 时是否拒绝。
- 定义“执行前确认”结构：`ConfirmRequest`（DSL、自然语言描述、预估影响行数或“未知”）、`ConfirmResponse`（Confirmed bool、Token string 或 ID 用于后续真正执行）；可在 `agent.Request.Metadata` 或专用类型中携带。
- 不在本任务内实现完整“等待用户确认”的流程，只定义结构并在 T-12 中使用。

**验收**：单元测试覆盖合法 SQL 通过、非法 SQL 或 readOnly 下写操作返回错误；ConfirmRequest/ConfirmResponse 可 JSON 序列化。

**依赖**：T-08，T-05（仅接口约定）。

---

## 五、P2：对话与编排

### T-10 数据对话会话上下文（当前数据源、上轮 DSL、表名等）

**目标**：扩展或新增会话上下文结构，保存数据源、上轮 DSL、当前表等，供多轮与改写使用；与现有 memory 兼容。

**参考代码**：`memory/buffer.go`、`memory/memory.go`、`agent/agent.go`（Request.Metadata）。

**实现要点**：

- 定义 `DataSessionContext` 结构体：`DatasourceID`、`DefaultSchema`、`LastDSL`、`LastTable`、`LastIntent` 等；可序列化为 JSON 存入 `memory.Entry.Metadata` 或单独 key。
- 提供 `GetDataContext(mem memory.Memory, sessionID string)` 与 `SetDataContext(...)` 的辅助函数，或通过 `agent.Request.Metadata["data_context"]` 在链中传递；若使用 BufferMemory，可在每轮回复后把上下文写入一条“系统”消息的 Metadata 或单独存储。
- 会话 ID：与现有 `Request.RequestID` 或会话标识一致；若当前框架无会话 ID，可在 `Request.Metadata["session_id"]` 中约定。

**验收**：单元测试：写入上下文后读出与预期一致；与 BufferMemory 集成时不影响原有 GetRecent 行为（可选）。

**依赖**：T-06（使用 Intent、表名等类型）；可选依赖 `memory`。

---

### T-11 DataQueryAgent：编排意图识别 → DSL 生成 → 执行 → 结果格式化

**目标**：实现一个符合 `agent.Agent` 接口的 DataQueryAgent，串联意图识别、DSL 生成、校验、执行与结果格式化；与现有 ChatAgent/ReActAgent 风格一致。

**参考代码**：`agent/agent.go`、`agent/react_agent.go`、`templates/chat.go`。

**实现要点**：

- 新建类型 `DataQueryAgent`，实现 `agent.Agent`：`Run(ctx, req *agent.Request) (*agent.Response, error)`。
- 依赖注入：意图识别器（`intent.Recognizer`）、DSL 生成器（`dsl.Generator`）、执行器（`executor.Executor`）、元数据存储（`metadata.Store`）、可选会话上下文（T-10）；通过构造函数或 Option 注入。
- 流程：从 `req.Messages` 取最后一条用户消息；若 `Metadata` 中有“确认通过”的标记则直接执行（T-12）；否则调用 Recognizer → 若需澄清则返回提示并写入 Response；否则 Generator 生成 DSL → Validator 校验 → 若为修改类且未确认则返回“待确认”及 ConfirmRequest 到 Response.Metadata；若已确认或为查询则 Executor.Execute → 将 Result 格式化为表格/文本写入 Response.Text，并可选写入 Response.Metadata（如行数、DSL）。
- 错误处理：意图失败、生成失败、执行失败分别映射到用户可读提示，并可使用 `errs`。
- 与事件：可选在关键步骤发布 `events`（意图完成、DSL 生成、执行完成），与现有 `events.Bus` 一致。

**验收**：单元测试用 mock Recognizer/Generator/Executor 跑通“查询”与“修改待确认”两条路径；至少一个端到端测试用内存 MySQL 或 mock 跑简单 SELECT。

**依赖**：T-07，T-08，T-09，T-05，T-10。

---

### T-12 修改操作二次确认与确认超时

**目标**：在 DataQueryAgent 中实现修改类操作的二次确认流程及超时与撤销约定。

**参考代码**：`agent/agent.go`、T-09（ConfirmRequest/ConfirmResponse）、`middleware`（超时可用 context）。

**实现要点**：

- 当意图为 Insert/Update/Delete 且 Validator 通过后，不直接执行，而是将 DSL、自然语言描述、可选影响行数写入 `agent.Response.Metadata["confirm"]`（或专用字段），并返回提示语“请确认是否执行…”。
- 下一轮请求若携带确认（如 `Metadata["confirm_token"]` 或 `Messages` 中用户回复“确认”），则再执行该 DSL；否则可视为取消或超时。
- 超时：会话上下文中记录“待确认 DSL”的时间戳；若超过约定时间（如 5 分钟）再收到同一会话的请求，可视为过期并清除待确认状态，要求用户重新发起修改请求。
- 与 T-11 的 DataQueryAgent 集成：在 Run 内判断上一轮是否留下待确认，本轮是否为确认指令，再决定执行或取消。

**验收**：单元测试覆盖“首次修改 → 返回待确认”“确认后执行”“超时后不再执行”；可选测试“用户说取消”清除待确认。

**依赖**：T-11，T-09。

---

## 六、P3：接口与安全

### T-13 数据查询 HTTP API（POST /data/chat，请求/响应格式）

**目标**：提供与现有 `sath serve` 风格一致的 HTTP 接口，供 Web/前端调用数据对话能力。

**参考代码**：`cli/serve.go`（/chat、RequestID、errs 映射）、`templates/chat.go`、`templates/from_config.go`。

**实现要点**：

- 在 `templates` 包中新增 `NewDataQueryHandler(dataAgent agent.Agent, mws ...middleware.Middleware) middleware.Handler`，内部直接调用 `dataAgent.Run(ctx, req)`。
- 在 `cli/serve.go` 或单独 `cmd/dataquery` 中注册路由：`POST /data/chat`；请求体：`{ "message": "...", "session_id": "..." }`；响应体：`{ "reply": "...", "data": { "rows": [...], "columns": [...] }, "confirm_required": false }`；若需确认则 `confirm_required: true` 且返回 `confirm_request` 对象（见 T-09）。
- 鉴权：本任务可仅预留 `Authorization` 或 `X-API-Key` 校验（返回 401）；具体 RBAC 在 T-15。
- 与现有 `/chat` 并存：数据对话走 `/data/chat`，普通对话仍走 `/chat`。

**验收**：使用 curl 或单元测试发 POST 请求，验证 200 与 JSON 结构；未授权时返回 401（若已实现预留）。

**依赖**：T-11，T-12。

---

### T-14 数据源配置加载与默认数据源选择

**目标**：从配置文件或环境变量加载数据源列表，并实现“未指定数据源时”的默认选择策略。

**参考代码**：`config/config.go`、`config.LoadWithEnv`、`datasource`（T-01）。

**实现要点**：

- 扩展 `config` 或在独立文件中定义数据源配置结构：如 `DataSources []datasource.Config`、`DefaultDatasourceID string`。
- 实现 `datasource.LoadFromFile(path string) ([]Config, DefaultID string, error)` 或从 `config.Config` 的扩展字段读取；支持从环境变量覆盖敏感项（如密码）。
- 在 DataQueryAgent 或 HTTP 层：若用户未指定且会话上下文中无数据源，则使用 `DefaultDatasourceID`；若未配置默认则返回提示“请指定数据源”或列出可选列表。
- 与 T-10 会话上下文结合：首次选择后写入会话上下文，后续同会话可沿用。

**验收**：单元测试：加载 YAML 得到 DataSources 与 DefaultID；集成测试：无指定时使用默认数据源执行一次简单查询。

**依赖**：T-01，T-11。

---

### T-15 权限占位与审计事件（RBAC 占位 + events 发布）

**目标**：为数据源访问与修改操作预留权限校验点，并发布审计事件，与框架 events 一致。

**参考代码**：`errs/errs.go`、`events/events.go`、`middleware`（链式）。

**实现要点**：

- 定义 `auth.Checker` 接口：`CanQuery(ctx, userID, datasourceID string) bool`、`CanExecute(ctx, userID, datasourceID string, dsl string) bool`（可选）；首版实现返回 true 的占位实现 `PermissiveChecker`。
- 在 DataQueryAgent 执行前调用 Checker：若不允许则返回 `errs.ErrUnauthorized` 或自定义错误，不执行 DSL。
- 审计：在关键步骤发布 `events.Event`（如意图识别完成、DSL 生成、执行请求、执行完成、确认执行）；Event 的 Payload 包含 session_id、user_id、datasource_id、意图类型、是否修改、DSL 摘要（可脱敏）；使用现有 `events.Bus` 与 `events.Kind`，可扩展新 Kind 如 `DataQueryIntent`、`DataQueryExecuted`。
- 日志：与 `obs` 或现有 LoggingMiddleware 结合，确保请求与执行可追溯。

**验收**：单元测试：Checker 返回 false 时 Agent 返回未授权错误；发布事件后通过订阅者能收到预期 Kind 与 Payload。

**依赖**：T-11，T-13（可选，用于获取 userID）。

---

## 七、P4：可观测与验收

### T-16 数据查询指标与 trace（意图耗时、DSL 耗时、执行耗时）

**目标**：为数据对话链路打点指标与 trace，与现有 obs 和 middleware 风格一致。

**参考代码**：`obs/metrics.go`、`middleware/metrics.go`、`obs/tracing.go`、`middleware/tracing.go`。

**实现要点**：

- 在 `obs` 包中新增或扩展：如 `ObserveDataIntent(duration, intent string)`、`ObserveDSLGenerate(duration)`、`ObserveDataExec(datasourceID, duration, status)`；使用 Prometheus Counter/Histogram，与现有 `agent_requests_total` 等区分（如用 label `handler="dataquery"`）。
- 在 DataQueryAgent 或包装其的中间件中，在意图识别、DSL 生成、执行三处分别记录耗时并调用上述 Observe；执行结果成功/失败写入 status。
- Trace：在现有 TracingMiddleware 或 DataQuery 专用中间件中，为一次 /data/chat 请求创建 span，并为意图识别、DSL 生成、执行创建子 span；与现有 `otel` 用法一致。

**验收**：调用若干次 /data/chat 后，/metrics 中能看到对应指标；trace 中能看到父子 span 关系（若已启用 tracer）。

**依赖**：T-11，T-13。

---

### T-17 端到端测试与 MVP 验收用例

**目标**：提供可自动运行的端到端用例与 MVP 验收清单，便于 Cursor Agent 或人工回归。

**参考代码**：`agent/agent_test.go`、`templates/templates_test.go`、`cmd/demo`。

**实现要点**：

- 在 `cmd/dataquery` 或 `templates` 下增加 e2e 测试：使用 testcontainer 启动 MySQL（或使用内存 SQL 驱动），配置一个数据源，构造 DataQueryAgent，发送 1～2 条自然语言查询（如“查询 users 表的前 5 条”），断言 Response 中包含预期列或行数。
- 文档中列出 MVP 验收清单：例如“(1) 配置 MySQL 数据源并拉取元数据；(2) 通过 /data/chat 发送‘有哪些表’并返回表列表；(3) 发送‘查询 xxx 表 limit 2’并返回 2 行；(4) 发送修改类请求并返回待确认，确认后执行成功”等。
- 若项目有 Makefile 或 CI，增加 `make e2e-dataquery` 或 `go test ./... -tags=e2e` 的说明。

**验收**：在本地或 CI 中运行 e2e 测试通过；MVP 清单逐条可执行并通过。

**依赖**：T-13，T-14，T-02，T-04，T-11。

---

## 八、任务列表（便于 Cursor Agent 逐条执行）

| 任务 ID | 标题                         | 包/主要文件           | 依赖    |
|---------|------------------------------|------------------------|---------|
| T-01    | 数据源驱动接口与连接配置     | datasource/            | -       |
| T-02    | MySQL 驱动实现               | datasource/mysql.go    | T-01    |
| T-03    | 元数据模型与存储接口         | metadata/              | -       |
| T-04    | MySQL 元数据拉取与缓存       | metadata/mysql.go      | T-02,T-03 |
| T-05    | 执行器接口与 MySQL 执行      | executor/              | T-01,T-02 |
| T-06    | 意图与实体数据结构           | intent/                | -       |
| T-07    | 意图识别器接口与 LLM 实现    | intent/                | T-06    |
| T-08    | NL2SQL 生成（MySQL）        | dsl/                   | T-06,T-03 |
| T-09    | DSL 校验与执行前确认结构     | dsl/                   | T-08,T-05 |
| T-10    | 数据对话会话上下文           | intent 或 memory 扩展  | T-06    |
| T-11    | DataQueryAgent 编排          | agent/ 或 dataquery/   | T-07,T-08,T-09,T-05,T-10 |
| T-12    | 修改操作二次确认与超时       | 同 T-11                | T-11,T-09 |
| T-13    | 数据查询 HTTP API            | templates/, cli/        | T-11,T-12 |
| T-14    | 数据源配置加载与默认选择     | config/, datasource/   | T-01,T-11 |
| T-15    | 权限占位与审计事件           | auth 或 middleware     | T-11,T-13 |
| T-16    | 数据查询指标与 trace         | obs/, middleware/      | T-11,T-13 |
| T-17    | 端到端测试与 MVP 验收        | 测试 + 文档            | T-13,T-14,T-02,T-04,T-11 |

---

## 九、使用说明（给 Cursor Agent）

- **单次执行**：每次只领取一个任务（如 T-01），按“参考代码”“实现要点”“验收”完成后再进行下一任务；有依赖的任务需等前置任务完成。
- **风格**：新包与现有 `agent`、`model`、`middleware`、`config`、`errs`、`events`、`obs` 保持同一风格：接口+实现、Option 配置、错误用 `errs`、可观测用 `obs`/`events`。
- **测试**：每个任务至少包含该包或该文件的单元测试；涉及多包协作的由后续端到端任务（T-17）覆盖。
- **文档**：新包在 `docs/api-reference.md` 或 `docs/concepts.md` 中增加简短说明；需求来源标注为《基于对话的数据查询与修改系统-完整需求文档》对应章节。
