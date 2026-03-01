# 数据查询 Agent — 开发任务拆解（Cursor Agent 版）

本文档基于 **《DATABASE_AGENT_REQUIREMENTS.md》**（工具驱动 ReAct 形态）将需求拆解为可由 **Cursor AI Agent** 逐条执行的开发任务，并保持与 **sath** 工程架构、代码风格一致。

---

## 一、需求与架构映射

| 需求章节 | 实现方式 | 主要包/模块 |
|----------|----------|--------------|
| 2.2 所需能力（列举/结构/只读/写改） | **工具**暴露给模型，执行层实现 | `tool` + `datasource`、`metadata`、`executor` |
| 2.1、2.3、2.4 思考→行动→观察、单步 | ReAct 循环 + 工具调用 | `agent.ReActAgent`、`model.ChatWithTools` |
| 2.5 安全、权限与确认 | 工具内部 + 执行层 | `auth`、会话内 pending + token、`executor` 只读拦截 |
| 2.6、3、4 | 配置、API、可观测 | `config`、`templates`、`cli`、`obs`、`events` |

**风格约定**：与现有 `agent`、`tool`、`model`、`memory`、`config`、`errs`、`events`、`obs` 一致；接口+实现、Option 配置、错误用 `errs`、可观测用 `obs`/`events`。

---

## 二、任务总览与依赖顺序

```
P0 基础设施（数据源、元数据、执行器）
  T-01 数据源接口与 Registry（可选复用现有 datasource 包）
  T-02 元数据 Store 与 MySQL 拉取（可选复用现有 metadata 包）
  T-03 执行器接口与 MySQL 执行（超时、行数、只读拦截）（可选复用现有 executor 包）

P1 数据查询工具（tool.Tool，对应需求 2.2）
  T-04 工具 list_tables（列举数据对象）
  T-05 工具 describe_table（查看结构/映射）
  T-06 工具 execute_read（执行只读请求）
  T-07 工具 execute_write（执行写/改：权限+确认 token，内部调执行器）

P2 确认与权限（需求 2.5）
  T-08 写/改确认 token 生成、绑定与一次性校验（会话内 pending）
  T-09 写/改前权限预检与审计事件（auth.Checker + events）

P3 ReAct 编排与系统提示（需求 2.3、2.4、4）
  T-10 数据查询系统提示与工作流（按数据源类型与只读/写改策略）
  T-11 DataQueryReActAgent：注册数据查询工具、注入会话/数据源上下文、封装 ReActAgent

P4 API 与配置（需求 3、4）
  T-12 数据查询 HTTP API（POST /data/chat）与数据源/默认数据源配置
  T-13 数据查询步骤指标与 E2E/MVP 验收
```

---

## 三、P0：基础设施

> 若工程中已有 `datasource`、`metadata`、`executor` 包且接口满足「列举/结构/只读执行/写执行」，本阶段可仅做**验收与适配**，不重复造轮子。

### T-01 数据源接口与 Registry

- **需求来源**：2.2 列举数据对象、4 数据源能力。
- **目标**：提供按 ID 获取数据源、Ping 的能力，供 list_tables / describe_table / 执行器使用。
- **参考代码**：`model/model.go`、`config/config.go`、`plugin/registry.go`。
- **实现要点**：包 `datasource`：`DataSource` 接口（`ID()`、`Ping(ctx)`、`Close()`）；`Config` 结构；`Registry`（`Register(cfg)`、`Get(id)`、`List()`）。MySQL 实现可选在本任务或 T-03 前完成。
- **验收**：`go build ./datasource/...` 通过；单测 Register/Get/List。
- **依赖**：无。

---

### T-02 元数据 Store 与 MySQL 拉取

- **需求来源**：2.2 列举数据对象、查看结构/映射。
- **目标**：提供按数据源获取 Schema/Table/Column 的能力，供 list_tables、describe_table 工具使用。
- **参考代码**：`memory/memory.go`、`parser/parser.go`。
- **实现要点**：包 `metadata`：`Schema`/`Table`/`Column`；`Store` 接口（`GetSchema(ctx, datasourceID)`、`Refresh(ctx, datasourceID, fetch)`、可选 `GetTable`）；`InMemoryStore`；MySQL 的 `FetchSchema` 与 `RefreshFromRegistry`（需能从未暴露的 DataSource 取 *sql.DB，如 MySQLDataSource.DB()）。
- **验收**：`go build ./metadata/...` 通过；单测 GetSchema/Refresh/GetTable（含 mock 或 sqlmock）。
- **依赖**：T-01（若 MySQL 拉取依赖 DataSource）。

---

### T-03 执行器接口与 MySQL 执行（超时、行数、只读拦截）

- **需求来源**：2.2 执行只读/写改、2.5 执行层只读边界。
- **目标**：提供执行 DSL（如 SQL）的能力，支持超时、最大行数、只读模式下拒绝写操作。
- **参考代码**：`datasource`、`errs/errs.go`。
- **实现要点**：包 `executor`：`Executor`（`Execute(ctx, datasourceID, dsl string, opts ExecuteOptions) (*Result, error)`）；`ExecuteOptions`（Timeout、MaxRows、ReadOnly、Params）；`Result`（Columns、Rows、AffectedRows、Error）。MySQL 实现：通过 datasource.Registry 取 MySQL；写操作前缀检测，ReadOnly 时返回错误；Query 用 ctx 超时、结果截断至 MaxRows。
- **验收**：单测只读拒绝写、MaxRows、超时与结果结构。
- **依赖**：T-01；可选 T-02（若执行器不依赖 metadata）。

---

## 四、P1：数据查询工具

> 每个工具均为 `tool.Tool`，`Execute(ctx, params map[string]any) (any, error)`。会话数据源 ID、用户 ID、只读/写改策略等通过 `context.Context` 或从请求注入的「会话上下文」中读取（见 T-11）。

### T-04 工具 list_tables（列举数据对象）

- **需求来源**：2.2 列举数据对象。
- **目标**：实现工具，返回当前数据源下的表/集合/索引列表。
- **参考代码**：`tool/tool.go`、`tool/calculator.go`、`metadata.Store`。
- **实现要点**：定义 `tool.Tool`：Name 如 `list_tables`，Description 与 Parameters（如 `datasource_id` 可选，默认从上下文取）；Execute 内从 ctx 或闭包取 `metadata.Store` 与当前 `datasource_id`，调用 `Store.GetSchema(ctx, datasource_id)`，将 `Schema.Tables` 转为可读文本或结构化 JSON 返回。若 Store 无缓存可先 `Refresh` 再 Get。
- **验收**：工具注册到 `tool.Registry` 后，Execute 在单测中返回表列表；参数与描述符合 JSON Schema 风格供模型解析。
- **依赖**：T-02。

---

### T-05 工具 describe_table（查看结构/映射）

- **需求来源**：2.2 查看结构/映射。
- **目标**：实现工具，返回指定表/对象的列与类型等信息。
- **参考代码**：`tool/tool.go`、`metadata.Store`。
- **实现要点**：Name 如 `describe_table`，Parameters 含 `table_name`（必填）、可选 `datasource_id`；Execute 调用 `Store.GetTable(ctx, datasource_id, table_name)` 或从 GetSchema 结果中按表名查找，返回列名、类型、可空、主键等描述。
- **验收**：单测传入 table_name 返回列信息；表不存在时返回明确错误。
- **依赖**：T-02。

---

### T-06 工具 execute_read（执行只读请求）

- **需求来源**：2.2 执行只读请求。
- **目标**：实现工具，执行只读 DSL（如 SELECT），返回结果集或错误。
- **参考代码**：`tool/tool.go`、`executor.Executor`。
- **实现要点**：Name 如 `execute_read`，Parameters 含 `dsl`（或 `query`）、可选 `datasource_id`；Execute 内从 ctx 取 datasource_id、Executor、ReadOnly=true；调用 `Executor.Execute(ctx, datasource_id, dsl, ExecuteOptions{ReadOnly: true, MaxRows, Timeout})`，将 Result 转为文本或结构化返回。执行层必须拒绝写操作。
- **验收**：单测传入合法 SELECT 返回行数据；传入 INSERT/UPDATE 返回明确错误。
- **依赖**：T-03。

---

### T-07 工具 execute_write（执行写/改：权限 + 确认 token）

- **需求来源**：2.2 执行写/改请求、2.5 权限与确认。
- **目标**：实现工具，支持「提议写/改」（返回待确认 + token）与「携带 token 确认执行」两种调用方式。
- **参考代码**：`tool/tool.go`、`executor`、`auth.Checker`、T-08 会话 pending。
- **实现要点**：Name 如 `execute_write`，Parameters 含 `dsl`，可选 `confirm_token`。  
  - 若未传 `confirm_token`：先 `Checker.CanExecute(ctx, userID, datasourceID, dsl)`；不通过则返回无权限；通过则生成唯一 token，将 (token, dsl, 时间) 写入会话 pending，返回「待确认」及 token 与操作描述，**不执行**。  
  - 若传入 `confirm_token`：从会话中校验 token 与 pending 匹配且未超时、一次性使用；通过则调用 `Executor.Execute(..., ReadOnly: false)`，执行后清除 pending，返回执行结果。  
  工具内需能访问会话存储（如 SessionStore）与 auth.Checker；userID/datasourceID 从 ctx 或请求上下文取。
- **验收**：单测「无 token → 权限通过 → 返回待确认 + token」「正确 token → 执行并清除」「错误/过期 token → 拒绝」。
- **依赖**：T-03、T-08、T-09。

---

## 五、P2：确认与权限

### T-08 写/改确认 token 生成、绑定与一次性校验

- **需求来源**：2.5 用户确认、取消或超时视为不执行。
- **目标**：在会话内维护「待确认写/改」状态，token 与 (dsl, 用户, 会话) 绑定，一次性有效，支持超时清除。
- **参考代码**：`memory`、`agent` 的 Request.Metadata。
- **实现要点**：定义会话内结构（如 `PendingWrite`：Token、DSL、CreatedAt）；提供 `SessionStore` 或扩展现有会话存储的 Get/Set；生成 token 时写入 PendingWrite；校验时匹配 token、未超时（如 5 分钟）、校验通过后删除 PendingWrite；超时则在下次读取会话时清除。可放在 `intent` 包（DataSessionContext.PendingConfirmToken 等）或独立 `dataquery` 包。
- **验收**：单测「生成 token → 校验通过 → 再次使用同一 token 失败」「超时后 pending 清除」。
- **依赖**：无（T-07 依赖本任务）。

---

### T-09 写/改前权限预检与审计事件

- **需求来源**：2.5 权限校验、执行结果可追溯。
- **目标**：写/改在进入确认前做权限检查；执行前后发布审计事件。
- **参考代码**：`auth/checker.go`、`events/event.go`。
- **实现要点**：`auth.Checker` 接口（`CanQuery`、`CanExecute`），至少实现占位 `PermissiveChecker`。在 execute_write 工具内：**提议阶段**先调用 `CanExecute`，不通过则直接返回，不写入 pending。审计：在工具执行写/改前后发布 `events.Event`（如 `dataquery.write.proposed`、`dataquery.write.executed`），Payload 含 user_id、session_id、datasource_id、操作摘要、结果、是否经确认等。
- **验收**：单测 Checker 返回 false 时工具不写入 pending；发布事件后 Payload 含约定字段。
- **依赖**：无（T-07 依赖本任务）。

---

## 六、P3：ReAct 编排与系统提示

### T-10 数据查询系统提示与工作流

- **需求来源**：2.3 系统行为约定、4 推理格式与提示。
- **目标**：提供可注入的系统提示模板，包含角色、工作流、请求规范、单步与格式约束、示例；支持按数据源类型与只读/写改策略切换。
- **参考代码**：`agent`、`templates`。
- **实现要点**：在 `templates` 或新建 `dataquery` 包中定义「系统提示」文本或模板：推荐工作流（探索→编写→校验与确认（写时）→执行→解读）；只读/写改边界；每轮单步、「思考」与「行动」格式；示例（列举表、查结构、只读查询、写改带确认）。可接受参数：数据源类型（如 mysql）、是否开放写改（bool），用于裁剪示例与约束说明。
- **验收**：单测或人工检查生成的提示包含上述要素；切换参数后只读/写改相关表述随之变化。
- **依赖**：无。

---

### T-11 DataQueryReActAgent：注册工具与会话上下文

- **需求来源**：2.1、2.4、4 实现形态（工具驱动 ReAct）。
- **目标**：组装数据查询专用 ReAct Agent：注册 list_tables、describe_table、execute_read、execute_write 工具；在每次 Run 时将会话数据源 ID、用户 ID、只读/写改策略注入 ctx 或工具闭包，供工具使用；使用现有 `agent.ReActAgent` 与 `model.ChatWithTools`。
- **参考代码**：`agent/react_agent.go`、`tool/tool.go`、`plugin/registry.go`。
- **实现要点**：新建 `DataQueryReActAgent` 或工厂函数：接收 `datasource.Registry`、`metadata.Store`、`executor.Executor`、`auth.Checker`、会话存储、配置（默认数据源、是否只读、MaxSteps 等）；在 Run 前根据 `Request.Metadata`（session_id、user_id、datasource_id）构建「请求级上下文」，创建或复用 `tool.Registry`，注册上述 4 个工具（工具 Execute 通过闭包或 ctx 访问 Store/Executor/Checker/会话）；将系统提示（T-10）与数据源类型、只读策略拼接为第一条 system message；调用 `ReActAgent.Run` 或内联 ReAct 循环（ChatWithTools → 注入 tool 结果 → 循环直到无工具调用或达 MaxSteps）。返回 Response 为模型最终答案。
- **验收**：单测：mock Store/Executor，发送「列出表」类消息，验证调用了 list_tables 且回复包含表列表；发送只读查询验证 execute_read 被调用。
- **依赖**：T-04～T-07、T-08、T-09、T-10。

---

## 七、P4：API 与配置

### T-12 数据查询 HTTP API 与数据源配置

- **需求来源**：3 使用方式、4 多数据源与配置。
- **目标**：暴露 POST /data/chat，请求体含 message、session_id 等；支持从配置加载数据源列表与默认数据源，并注入到 Agent/工具上下文。
- **参考代码**：`cli/serve.go`、`templates/chat.go`、`config/config.go`。
- **实现要点**：在 `config` 中扩展 `DataSources`、`DefaultDatasourceID`（若尚未存在）。在 serve 或 `templates` 中：根据 config 构建 DataQueryReActAgent（T-11），注册路由 POST /data/chat；请求体解析 message、session_id；构造 `agent.Request`（Messages、Metadata 含 session_id、user_id、可选 datasource_id）；调用 Agent.Run；响应 JSON 含 reply，若涉及写/改待确认可含 confirm_required、confirm_request（含 token）。未配置数据源时可选返回 503 或跳过注册。
- **验收**：curl 或单测 POST /data/chat 返回 200 与 reply；配置 DataSources 后 Agent 能使用对应数据源。
- **依赖**：T-11、T-01、T-02、T-03。

---

### T-13 数据查询步骤指标与 E2E/MVP 验收

- **需求来源**：2.6 可观测性、整体验收。
- **目标**：为数据查询链路打点（步骤耗时、成功/失败）；提供 E2E 测试与 MVP 验收清单。
- **参考代码**：`obs/metrics.go`、`middleware/metrics.go`、现有 E2E 模式。
- **实现要点**：在 `obs` 中增加数据查询相关指标（如 dataquery_tool_calls_total、dataquery_step_duration_seconds），在工具执行或 ReAct 步骤中打点。E2E：启动或 mock 服务，发送「有哪些表」「查询某表 limit 2」等，断言回复与状态码。文档：MVP 验收清单（配置数据源 → 问表列表 → 只读查询 → 写改提议 → 确认执行）与需求 2.2、2.5 对应。
- **验收**：`go test ./...` 含 dataquery 相关通过；/metrics 可见 dataquery 指标；MVP 清单可执行。
- **依赖**：T-12。

---

## 八、任务列表（便于 Cursor Agent 领取）

| 任务 ID | 标题 | 包/主要文件 | 依赖 |
|---------|------|-------------|------|
| T-01 | 数据源接口与 Registry | datasource/ | — |
| T-02 | 元数据 Store 与 MySQL 拉取 | metadata/ | T-01 |
| T-03 | 执行器与 MySQL 执行（超时、行数、只读） | executor/ | T-01 |
| T-04 | 工具 list_tables | tool/ 或 dataquery/ | T-02 |
| T-05 | 工具 describe_table | tool/ 或 dataquery/ | T-02 |
| T-06 | 工具 execute_read | tool/ 或 dataquery/ | T-03 |
| T-07 | 工具 execute_write（权限+确认） | tool/ 或 dataquery/ | T-03,T-08,T-09 |
| T-08 | 写/改确认 token 与一次性校验 | intent/ 或 dataquery/ | — |
| T-09 | 权限预检与审计事件 | auth/, events/ | — |
| T-10 | 数据查询系统提示与工作流 | templates/ 或 dataquery/ | — |
| T-11 | DataQueryReActAgent 组装 | agent/ 或 dataquery/ | T-04～T-10 |
| T-12 | 数据查询 HTTP API 与配置 | cli/, config/, templates/ | T-11,T-01～T-03 |
| T-13 | 指标与 E2E/MVP 验收 | obs/, 测试与文档 | T-12 |

---

## 九、使用说明（给 Cursor AI Agent）

- **单次执行**：每次只领取一个任务（如 T-01），按「实现要点」「参考代码」完成后再进行下一任务；有依赖的任务需等前置完成。
- **风格**：与现有 `agent`、`tool`、`model`、`memory`、`config`、`errs`、`events`、`obs` 保持一致；新代码优先放在现有包内，若需新包（如 `dataquery`）则与根目录下各包同级。
- **需求追溯**：每个任务在文档中标注了「需求来源」；实现时请对照《DATABASE_AGENT_REQUIREMENTS.md》中的工作流与安全约束。
- **测试**：每个任务至少包含该包或该文件的单元测试；多包协作由 T-13 E2E 覆盖。
