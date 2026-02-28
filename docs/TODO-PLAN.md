# 未实现功能计划文档

本文档基于对当前工程与 [plan.md](../plan.md) 的对照，将**尚未实现或未完全达标**的功能整理为可执行计划，便于按阶段推进。

---

## 一、总览

| 阶段 | 范围 | 建议周期 |
|------|------|----------|
| **阶段 A** | V0.1 收尾（可选） | 约 0.5–1 周 |
| **阶段 B** | V0.2 缺口：模型/配置/记忆/中间件/可观察性/文档 | 约 3–5 周 |
| **阶段 C** | V0.3 缺口：多模态/国内模型/TLS/压测与覆盖率 | 约 2–4 周 |

---

## 二、阶段 A：V0.1 收尾（可选）

> 目标：补全 plan 中 V0.1 所列的“剩余可选收尾事项”。

| 序号 | 需求来源 | 任务描述 | 优先级 |
|------|----------|----------|--------|
| A.1 | V0.1 测试补强 | 为 `agent.ChatAgent` 增加更多单元测试（含/无记忆、模型出错等场景） | P2 |
| A.2 | V0.1 测试补强 | 为 `tool.Registry` 和 `ChatWithTools` 增加基本集成测试（fake model 或打桩） | P2 |
| A.3 | V0.1 HTTP Demo | 新增 `cmd/http_demo`（可选；当前 `sath serve` 已提供 POST /chat，可视为已覆盖） | P3 |

**说明**：若以 `sath serve` 作为标准 HTTP 入口，A.3 可标记为“不实施”。

---

## 三、阶段 B：V0.2 未实现项

### B.1 模型与多模型策略（F1.2–F1.3, F1.5）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| B.1.1 | F1.5 | **国内模型适配器**：至少接入一家国内厂商（阿里通义千问 或 百度文心一言），一期仅文本 Chat/Generate，通过 `model.RegisterProvider` 注册，支持 `NewFromIdentifier("dashscope/...")` 或 `"qianfan/..."` | P0 |
| B.1.2 | F1.3 | **多模型策略扩展**：在 `model` 包中增加 `by_cost`、`by_latency` 等策略（或 tag 选择逻辑），并在 `MultiModel` / `CallConfig` 中支持按成本或延迟标签选模型 | P1 |

### B.2 配置化（F4.2）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| B.2.1 | F4.2 | **环境变量覆盖**：`config.Load()` 之后支持用环境变量覆盖部分字段（如 `OPENAI_MODEL` 覆盖 `Config.ModelName`），可提供 `ApplyEnvOverrides(cfg *Config)` 或 `LoadWithEnv(path string) (Config, error)` | P0 |
| B.2.2 | F4.2 | **多配置文件**：支持按环境加载不同文件（如 `config.dev.yaml` / `config.prod.yaml`），或通过 `LoadWithEnv("config."+env+".yaml")` 由环境变量指定路径 | P1 |
| B.2.3 | F4.2 | **配置驱动 Agent 装配**：根据 `Config`（模型标识、MaxHistory、中间件列表等）自动构建 Agent 与中间件链，便于 `sath serve -c config.yaml` 等场景一键装配 | P1 |

### B.3 记忆与 RAG（F2.5）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| B.3.1 | F2.5 | **向量库后端**：在 `memory` 包中增加至少一种持久化向量库实现（**pgvector** 或 **Chroma**），实现 `VectorStore` 接口，并在文档中说明依赖与配置 | P1 |

### B.4 输出解析器（F2.7）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| B.4.1 | F2.7 | **表格解析器**：在 `parser` 包中新增表格解析器（如将 Markdown 表格或简单文本表格解析为 `[][]string` 或结构化类型），实现 `Parser` 接口，并在 api-reference 中补充 | P1 |

### B.5 中间件（F3.3, F3.5）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| B.5.1 | F3.5 | **中间件顺序/优先级**：在注册或构建时支持 Order/优先级（例如中间件结构体带 `Order int`，或提供 `ChainBuilder` 按优先级排序后再 `Chain`） | P1 |
| B.5.2 | F3.3 | **全局与局部中间件**：在 Agent/Server 构建时支持「全局链 + Agent 局部链」的合并策略（如先执行全局中间件，再执行该 Agent 专属中间件），并在模板或 serve 中提供示例 | P1 |

### B.6 可观察性（F5.2）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| B.6.1 | F5.2 | **Token 消耗指标**：在 `obs` 包中增加 Prometheus 指标（如 `agent_tokens_total` 或按 input/output 分列），在模型调用返回 token 用量时由中间件或适配器上报 | P1 |

### B.7 文档与可用性（5.1, 5.2）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| B.7.1 | 5.1 | **英文版 README**：新增 `README.en.md` 或独立英文 README，覆盖 Quick Start、核心概念、CLI、基本示例，与中文 README 结构对应 | P1 |
| B.7.2 | 5.2 | **统一错误与装配**：在关键 HTTP/Agent 路径中统一使用 `errs` 包返回；可选：提供 `templates.NewAgentFromConfig(cfg config.Config)` 或类似“从配置装配 Agent”的入口 | P2 |

---

## 四、阶段 C：V0.3 未实现项

### C.1 多模态与本地模型（F1.5, F1.6）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| C.1.1 | F1.5 | **多模态模型适配器**：为 Claude-3 Vision（或其它多模态 API）提供适配器，实现 `Model` 接口并对多模态 `Message.Parts` 做格式转换；在模板或 demo 中增加「多模态理解 Agent」示例 | P1 |
| C.1.2 | F1.6 | **Llama.cpp 客户端**：设计可选的 `LocalModel` 接口，实现 Llama.cpp 客户端（或通过 build tags 集成），并在模型工厂中支持配置启用 | P2 |

### C.2 安全与运维（NF5）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| C.2.1 | NF5 | **HTTP 服务 TLS**：在文档（如 best-practices 或 README）中提供“以 TLS 运行 sath serve”的配置示例（如反向代理 + 证书，或 Go 标准库 ListenTLS 示例） | P1 |

### C.3 测试与性能（NF1, NF6）

| 序号 | 需求 ID | 任务描述 | 优先级 |
|------|---------|----------|--------|
| C.3.1 | NF6 | **覆盖率达标**：运行 `go test -cover ./...`，将单元测试覆盖率提升至 ≥85%，对薄弱包补充用例 | P1 |
| C.3.2 | NF1 | **100 QPS 压测**：编写压测脚本或文档（如基于 hey/wrk 对 `POST /chat`、`GET /health` 的压测步骤），并在文档中记录或验证 P99 延迟等 NF1 指标 | P1 |
| C.3.3 | NF6 | **关键路径 Benchmark**：为工具调用、向量检索等增加 `_test.go` 中的 benchmark，便于评估变更对延迟的影响 | P2 |

---

## 五、执行建议

1. **依赖关系**：建议先做 B.2.1（环境变量覆盖）、B.1.1（国内模型），再做 B.2.3（配置装配）和 B.6.1（token 指标）。
2. **优先级**：P0 优先，P1 次之，P2/P3 在时间允许时补齐。
3. **验收**：每项完成后在本文档中标注状态（如 `[x]`），并在 plan.md 或 CHANGELOG 中更新对应需求 ID。
4. **与 plan.md 对齐**：本文档中的需求 ID（F1.x、F2.x 等）与 [plan.md 需求映射表](../plan.md#需求-id--任务映射表) 一致，便于追溯。

---

## 六、状态追踪（可选）

在下方按“阶段.序号”标注完成情况，便于跟踪进度。

### 阶段 A
- [ ] A.1  ChatAgent/Registry/ChatWithTools 测试补强
- [ ] A.2  cmd/http_demo（可选）
- [ ] A.3  （若不做 A.2 可标为 N/A）

### 阶段 B
- [x] B.1.1  国内模型（通义/文心）：dashscope 适配器，NewFromIdentifier("dashscope/qwen-turbo")
- [x] B.1.2  多模型策略 by_cost / by_latency：StrategyByCost、StrategyByLatency，MultiModelWithOptions
- [x] B.2.1  环境变量覆盖：ApplyEnvOverrides、LoadWithEnv
- [x] B.2.2  多配置文件：LoadForEnv(env, dir)
- [x] B.2.3  配置驱动 Agent 装配：NewChatAgentHandlerFromConfig、DefaultMiddlewareMap
- [ ] B.3.1  pgvector/Chroma 向量库
- [x] B.4.1  表格解析器：parser.TableParser
- [x] B.5.1  中间件 Order/优先级：OrderedMiddleware、ChainBuilder
- [x] B.5.2  全局+局部中间件：MergeGlobalLocal
- [x] B.6.1  Token 消耗指标：obs.ObserveTokenUsage、agent_tokens_total
- [x] B.7.1  英文 README：README.en.md
- [x] B.7.2  统一错误与装配：serve 使用 errs、FromConfig 装配

### 阶段 C
- [ ] C.1.1  多模态适配器与示例
- [ ] C.1.2  Llama.cpp / LocalModel
- [x] C.2.1  TLS 配置示例：docs/best-practices.md §7
- [ ] C.3.1  覆盖率 ≥85%
- [x] C.3.2  100 QPS 压测：docs/loadtest.md
- [x] C.3.3  工具/向量 benchmark：tool_bench_test.go、memory/vector_bench_test.go

---

*文档版本：基于 plan.md 与当前代码库对照整理，随实现进展可增删任务项。*
