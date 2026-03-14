# Sath 框架最佳实践与使用指南

本文档基于对全工程的梳理，给出配置、Agent、Skills、工具、事件、中间件与部署等方面的**详细最佳实践**，便于正确、安全、可维护地使用本框架。

---

## 1. 工程概览与目录结构

### 1.1 核心模块

| 目录/包 | 职责 |
|--------|------|
| `agent/` | Agent 接口与实现：`ChatAgent`（纯对话）、`ReActAgent`（工具调用循环）、`PlanExecuteAgent` |
| `model/` | 模型抽象与多厂商实现（OpenAI、Ollama、DashScope 等），含 `ChatWithTools` 的 Function Calling 桥接 |
| `memory/` | 短期记忆（BufferMemory）、向量记忆、摘要记忆及 Manager |
| `tool/` | 工具定义（Tool/Registry）、内置工具（calculator、load_skill、execute_skill_script、http_request 等）、MCP 与数据查询工具 |
| `skills/` | Skill 索引（Index）、SKILL.md 解析、LoadSkillBody/LoadSkillFile |
| `templates/` | 从配置装配 Handler：`NewChatAgentHandlerFromConfig`、`NewSkillsAwareChatHandlerFromConfig`、`NewDataQueryHandlerFromConfig` |
| `config/` | 配置结构（Config、SkillsConfig、DataSources）、Load/LoadWithEnv/LoadForEnv、环境变量覆盖 |
| `events/` | 事件总线（Bus）、事件类型（RunStarted、ModelInvoked、ToolInvoked、ToolExecuted 等） |
| `middleware/` | 中间件链（Recovery、Logging、Metrics、Tracing、RateLimit、ContentSafety、Debug 等） |
| `obs/` | 可观测性：日志、指标（Prometheus）、健康检查、Tracer 初始化 |
| `datasource/` | 数据源抽象与实现（MySQL、Elasticsearch、MongoDB、Hive 等），供数据查询 Agent 使用 |
| `cli/` | 统一 CLI（sath init / demo / serve） |

### 1.2 入口与示例

- **CLI 入口**：`cmd/sath/main.go` → `cli.NewRootCommand().Execute()`
- **HTTP 服务**：`sath serve` 使用 `cli/serve.go`，提供 `POST /chat`、`POST /data/chat`、`GET /health`、`GET /metrics`
- **示例**：`examples/skills_agent`（Skills + ReAct）、`examples/database_agent/*`（数据查询）、`cmd/demo`、`cmd/tool_demo`

---

## 2. 配置最佳实践

### 2.1 配置文件格式与加载

- 支持 **YAML**（`.yaml`/`.yml`）与 **JSON**；推荐 YAML 便于注释与分层。
- 加载方式：
  - `config.Load(path)`：仅文件；
  - `config.LoadWithEnv(path)`：文件 + 环境变量覆盖（推荐）；
  - `config.LoadForEnv(env, dir)`：按环境加载，如 `config.dev.yaml`。
- 环境变量覆盖见 `config.ApplyEnvOverrides`（如 `OPENAI_MODEL`、`AGENT_MAX_HISTORY`、`AGENT_MIDDLEWARES`、`DEFAULT_DATASOURCE_ID`、`DATA_ALLOW_WRITE`）。

### 2.2 核心配置项

```yaml
model: openai/gpt-4o          # 模型标识，与 model.NewFromIdentifier 对应
max_history: 10               # 短期记忆条数
middlewares:                  # 中间件链顺序
  - logging
  - metrics
  - tracing
```

### 2.3 Skills 配置

```yaml
skills:
  skills_dirs: ["skills", "skills_examples"]   # Skill 目录，扫描其下 SKILL.md
  enabled_skills: []                           # 白名单，非空时仅启用列出的 Skill
  disabled_skills: []                          # 黑名单
  allow_script_execution: true                 # 是否允许 execute_skill_script
  script_allowed_extensions: [".sh", ".py", ".js"]
  script_timeout_seconds: 60                   # 单次脚本超时（秒），建议 ≤300
  mcp_servers:                                 # MCP 服务列表，供 load_skill 时按 Skill 声明注册
    - endpoint: "http://..."
      id: "my-mcp"
      backend: "stdio"  # 或 sse 等
```

**最佳实践**：

- 生产环境若不需要执行脚本，将 `allow_script_execution` 设为 `false`；需要时再显式开启并配合 `script_allowed_extensions` 白名单。
- 脚本超时不宜过大，避免长时间占用；默认 30 秒，可按需调到 60–300。

### 2.4 数据源配置（数据查询 Agent）

当使用 `NewDataQueryHandlerFromConfig` 时，需在配置中提供 `data_sources`：

```yaml
data_sources:
  - id: my-mysql
    type: mysql
    # ... 连接等配置
default_datasource_id: my-mysql
data_allow_write: false   # 为 true 时开放写/改工具（需权限与确认流程）
```

### 2.5 密钥与敏感信息

- **禁止在配置文件或代码中硬编码 API Key、密码**；一律从环境变量或外部密钥管理读取。
- 例如：`OPENAI_API_KEY`、`OPENAI_BASE_URL` 等由模型实现按需读取。

---

## 3. Agent 使用最佳实践

### 3.1 选择 Agent 类型

- **纯对话、无工具**：使用 `ChatAgent` + `NewChatAgentHandlerFromConfig` 或 `NewChatAgentHandler`。
- **需要工具调用（含 Skills、MCP、自定义工具）**：使用 **ReActAgent**；通过 `templates.NewSkillsAwareChatHandlerFromConfig` 可获得「Skills 摘要 + load_skill / read_skill_file / execute_skill_script」的 ReAct 对话。
- **数据查询（MySQL/ES 等）**：使用 `NewDataQueryHandlerFromConfig`，内部为 ReActAgent + 数据源相关工具（list_tables、describe_table、execute_read 等）。

### 3.2 Request 与 RequestID

- 每次调用建议设置 `RequestID`，便于日志、事件与追踪关联；若为空，框架会在 Run 内生成。
- 在 HTTP 或中间件层可从 `X-Request-ID` 读取并写入 `req.RequestID`；若使用事件或 Trace，需将同一 ID 写入 `context`（如 `tool.ContextKeyRequestID`），以便工具层发布事件时带上 RequestID。

```go
if req.RequestID != "" {
    ctx = context.WithValue(ctx, tool.ContextKeyRequestID, req.RequestID)
}
return react.Run(ctx, req)
```

### 3.3 中间件顺序

- 建议顺序：**Recovery → Logging → （业务）→ Metrics / Tracing**；Debug 可包在最外层便于开发排障。
- `templates` 装配时已默认在业务 Handler 前加 Recovery + Logging，再按 `cfg.Middlewares` 追加；自定义中间件通过 `middlewareByName` 传入。

---

## 4. Skills 最佳实践

### 4.1 Skill 目录与 SKILL.md

- 每个 Skill 一个目录，内含 **SKILL.md**（必需）；可含 `scripts/`、`docs/`、`assets/` 等。
- 框架通过 `skills.NewIndex(dirs, enabled, disabled)` 扫描目录下所有 `SKILL.md`，解析 **YAML frontmatter** 构建索引。

### 4.2 Frontmatter 规范

```yaml
---
name: my-skill                 # 唯一标识，推荐 kebab-case
description: 简短描述，说明何时使用
tags: [database, analysis]
allowed_tools:                 # 可选，声明允许使用的工具名
  - load_skill
  - read_skill_file
  - execute_skill_script
---
```

- `name` 会出现在系统提示的 Skill 列表中；**Skill 名称是参数，不是工具名**，模型应调用 `load_skill(name)` 等工具，而不是把 name 当工具调用。
- 正文为 Markdown，写清工作流、步骤、示例及「脚本执行被禁用时」等异常处理说明。

### 4.3 三个核心工具

| 工具 | 用途 | 典型用法 |
|------|------|----------|
| `load_skill` | 按 name 加载完整 SKILL.md 正文 | 任务与某 Skill 相关时先调用，再按正文执行 |
| `read_skill_file` | 读取 Skill 目录下文件（如 docs/、assets/） | 仅读文件内容，不执行 |
| `execute_skill_script` | 执行 Skill 下 `scripts/` 内脚本 | 需配置 `allow_script_execution: true` 且 path 以 `scripts/` 开头 |

### 4.4 execute_skill_script 使用要点

- **path**：相对 Skill 根目录，且必须以 `scripts/` 开头，例如 `scripts/index.js`、`scripts/run.sh`。
- **input**（可选）：字符串，会作为 **stdin** 传入脚本；Node/Python 脚本可从 stdin 读 JSON 并执行逻辑，结果打印到 stdout。
- 支持多种运行时：`.sh` → sh，`.py` → python，`.js` → node；允许的扩展名由配置 `script_allowed_extensions` 控制。
- **不要事先假定「脚本执行已禁用」**：仅在实际调用该工具且返回「脚本执行已禁用」类错误时，再向用户说明需开启 `skills.allow_script_execution`。

### 4.5 Node.js 脚本可执行入口

若脚本为 `module.exports = async function (context) { ... }`，直接 `node index.js` 不会执行。需在脚本末尾增加「直接运行」分支：从 stdin（或 argv）读入 JSON，调用导出函数，将结果 `JSON.stringify` 输出到 stdout，这样 Go 的 `exec` 才能拿到输出。本仓库中 `vm-prelaunch-failure-investigator/scripts/index.js` 已按此方式实现。

### 4.6 不编造结果

系统提示已约束：当缺少必要信息、无法访问外部系统或脚本执行被禁用时，**不要凭空编造具体结果**（如版本号、数量、精确日志）；应如实说明受限原因，可给一般性排查建议，但须明确非基于真实执行结果的结论。

---

## 5. 工具（Tools）最佳实践

### 5.1 注册与 EventBus

- 使用 `tool.NewRegistry()` 创建注册表；若需事件追踪，调用 `reg.SetEventBus(events.DefaultBus())`，此后每次工具执行会发布 `ToolInvoked`（执行前）与 `ToolExecuted`（执行后，含 input/output）。
- 自定义工具通过 `reg.Register(tool.Tool{...})` 注册；参数建议用 JSON Schema 风格（`type`、`properties`、`required`），便于模型正确传参。

### 5.2 内置工具一览

- **load_skill / read_skill_file / execute_skill_script**：由 `tool.RegisterLoadSkillTool`、`RegisterExecuteSkillScriptTool` 等注册，Skills Handler 中已集成。
- **http_request**：`tool.RegisterHTTPTool(reg)`，支持 method、url、headers、body、timeout_seconds；发布 `HttpRequestInvoked` / `HttpRequestCompleted` 事件。
- **calculator_add**：示例计算工具。
- 数据查询场景：`list_tables`、`describe_table`、`execute_read`、`execute_write` 等由数据查询模板按数据源注册。

### 5.3 MCP 工具

- MCP 服务在配置的 `skills.mcp_servers` 中声明；当模型通过 `load_skill` 加载的 Skill 声明了 `mcp_servers` 时，框架会按配置将对应 MCP 工具注册到当前请求的 Registry，从而实现「按 Skill 按需挂载 MCP」。

---

## 6. 事件与可观测性

### 6.1 事件类型与顺序

- Run 生命周期：`RunStarted` → `ModelInvoked` → `ModelResponded` → （可能多轮）→ `ToolInvoked` → `ToolExecuted` → … → `RunCompleted` 或 `RunError`。
- 所有事件建议带 `RequestID`，便于按请求串联；Payload 统一 `map[string]any`，字段小写+下划线（如 `message_count`、`text_length`、`tool`、`input`、`output`）。

### 6.2 订阅事件

- 使用 `events.DefaultBus()` 或自建 `events.NewBus()`；在进程启动时可通过 `events.SetDefaultBus(bus)` 设置默认总线。
- `bus.Subscribe(async, listener)`：`async=true` 时异步执行，不阻塞发布方；可用于打日志、审计、指标上报。

### 6.3 追踪（Tracing）

- 开发环境可用 `obs.InitTracer(serviceName)` 初始化 stdout exporter，便于查看调用链。
- 中间件 `middleware.TracingMiddleware` 为每次 Agent.Run 创建 span；工具层若需为每次工具调用建 span，可在工具 Execute 内使用 `otel.Tracer(...).Start(ctx, ...)` 并设置属性与 RecordError。

---

## 7. 中间件与安全

### 7.1 推荐中间件链

- **Recovery**：必须，防止 panic 导致进程退出。
- **Logging**：推荐，记录请求/响应摘要与错误。
- **Metrics**：生产推荐，配合 `GET /metrics` 暴露 Prometheus 指标。
- **Tracing**：按需，与上游 Trace 系统对接时使用。
- **RateLimit**：按需，防止滥用。
- **ContentSafety**：涉及用户生成内容或敏感输出时建议启用。
- **Debug**：仅开发/排障时启用，生产关闭。

### 7.2 安全要点

- 不硬编码密钥；最小权限原则；对写/改类数据操作做权限校验与用户确认（见 DATABASE_AGENT_REQUIREMENTS 与数据查询相关文档）。
- 生产 HTTP 建议通过反向代理（Nginx/Caddy）做 TLS 终结，或使用 `ListenAndServeTLS` 并从环境变量读取证书路径。

---

## 8. 运行与部署

### 8.1 CLI

```bash
go build -o sath ./cmd/sath
./sath init -d myapp        # 初始化项目骨架
./sath demo                # 对话 REPL（需 OPENAI_API_KEY）
./sath serve -a :8080 -c config.yaml -d   # HTTP 服务，-d 为 debug
```

### 8.2 HTTP API

- **POST /chat**：Body `{"message":"用户输入"}`，可选 Header `X-Request-ID`；返回 `{"reply":"..."}`，debug 时含 `request_id`。
- **POST /data/chat**：数据查询 Agent，需配置数据源；Body 可含 `session_id`、`user_id`、`datasource_id` 等。
- **GET /health**：健康检查；**GET /metrics**：Prometheus 指标。

### 8.3 健康检查与超时

- 在 HTTP 层为每次请求设置合理的 `context.Context` 超时，避免长时间占用。
- 暴露 `/health` 并对模型、向量库等依赖做轻量探测（见 `obs.HealthHandler`）。

---

## 9. 常见问题与约束小结

| 问题 | 建议 |
|------|------|
| 模型把 Skill 名当工具调用 | 系统提示已说明「Skill 名称只是参数」；在 SKILL 正文中也可再次强调先 `load_skill(name)` 再按说明调用工具。 |
| 未执行工具就报「脚本执行已禁用」 | 系统提示已约束：仅在实际调用 `execute_skill_script` 且收到禁用错误后才给出该结论。 |
| 未执行工具就编造答案 | 系统提示已约束：缺少信息或无法执行时不得编造具体结果，应如实说明并可选给通用建议。 |
| execute_skill_script 报 only scripts under scripts/ | path 必须是以 `scripts/` 开头的相对路径，如 `scripts/index.js`。 |
| Node 脚本无输出 | 脚本需具备「直接运行」入口：读 stdin/argv，调主逻辑，结果写 stdout。 |
| 事件无 RequestID | 在调用 Agent 前设置 `req.RequestID`，并将该值写入 context 的 `tool.ContextKeyRequestID`。 |

---

## 10. 参考文档

- [Quick Start](quickstart.md)  
- [Concepts](concepts.md)  
- [Extending（插件与事件）](extending.md)  
- [Event Requirements](event_requirements.md)  
- [Skills Requirements](skills-requirements.md)  
- [Database Agent / DataQuery 需求](DATABASE_AGENT_REQUIREMENTS.md)  
- [CONTRIBUTING](../CONTRIBUTING.md)  

以上内容覆盖配置、Agent、Skills、工具、事件、中间件与部署的完整使用与约束，按此实践可减少误用并便于维护与扩展。
