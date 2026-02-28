## Best Practices

### 1. 模型与工具

- **明确模型职责**：将「纯对话」与「带工具调用」分开配置，避免一个 Agent 同时承担过多角色。
- **优先使用 tools API**：对于需要调用外部系统的场景，优先通过 `ChatWithTools` / tools API，而不是让模型输出可执行代码或自由格式命令。
- **控制温度与长度**：在重要生产路径上使用偏低温度（如 0.2–0.5），并设置合理的 MaxTokens。

### 2. 记忆与 RAG

- **短期记忆要有限**：`MaxHistory` 不宜过大，建议在 10–50 之间，根据业务调优。
- **向量记忆分层管理**：将「用户会话级别」与「全局知识库」区分开，减少检索噪音。
- **摘要要留关键信息**：在设计摘要 prompt 时强调保留结论、实体、约束条件，避免「只保留背景」。

### 3. 中间件与可观察性

- **默认启用日志与恢复**：在所有 Agent 链路前务必加上 `RecoveryMiddleware` 和日志中间件。
- **为重要 Agent 命名**：通过 `req.Metadata["agent_name"]` 传入 Agent 名称，方便在 Prometheus 和 trace 中区分。
- **对慢调用设置超时**：在上层（HTTP/CLI）为每次调用设置合理的 `context.Context` 超时。

### 4. 安全与合规

- **不要硬编码密钥**：所有 API Key 和密码从环境变量或外部配置管理系统中读取。
- **输入输出过滤**：对涉及敏感数据的场景启用 `ContentSafetyMiddleware`，或接入更严格的审核服务。
- **最小权限原则**：在调用外部 HTTP/API 工具时，确保下游服务也执行认证与授权检查。

### 5. 健康检查与调试（F5.4–F5.5）

- **暴露 /health**：在 HTTP 服务中挂载 `obs.HealthHandler(checks)`，对模型 API、向量库等依赖做轻量探测（如短超时 Generate 或 Search），便于负载均衡与运维巡检。
- **调试模式**：生产环境关闭 `--debug`；开发或排障时开启，可获取 `request_id`、脱敏的请求/响应长度以及错误时的调用栈，便于追踪与复现问题。

### 6. 测试与调试（NF6）

- **优先写单元测试**：对 Agent 的核心逻辑（如 ReActAgent、PlanExecuteAgent、记忆管理器）保持较高测试覆盖率；目标覆盖率见项目 NF6（如 ≥85%）。
- **基准测试**：对中间件链、工具调用等关键路径使用 `go test -bench` 做基准测试（如 `middleware` 包内 `BenchmarkChain_*`），便于评估变更对延迟的影响。
- **覆盖率**：运行 `go test -cover ./...` 或 `go test -coverprofile=cover.out ./...` 查看覆盖率，并针对薄弱包补充用例。
- **善用 stdout trace**：开发阶段使用 `obs.InitTracer` 的 stdout exporter，先在本地确认调用链是否符合预期，再接入生产级后端（Jaeger/Zipkin 等）。
- **分环境配置**：使用不同的配置文件（如 `config.dev.yaml` / `config.prod.yaml`）区分模型版本、限流策略与日志等级。

### 7. HTTP 服务 TLS（NF5 / C.2.1）

生产环境建议通过 TLS 暴露 `sath serve`，避免明文传输。

**方式一：反向代理（推荐）**

使用 Nginx/Caddy 等做 HTTPS 终结，后端 `sath serve` 仅监听 localhost：

```bash
sath serve -a 127.0.0.1:8080
```

在 Nginx 中配置 `proxy_pass http://127.0.0.1:8080` 并启用 `ssl_certificate` / `ssl_certificate_key`。

**方式二：Go 标准库 ListenTLS**

若需进程内 TLS，可自行包装 `http.Server`：

```go
srv := &http.Server{Addr: ":443", Handler: mux}
err := srv.ListenAndServeTLS("cert.pem", "key.pem")
```

证书可通过 Let's Encrypt、云厂商或自签生成；密钥与证书路径应从环境变量或配置读取，禁止硬编码。

