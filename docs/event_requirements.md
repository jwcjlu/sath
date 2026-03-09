# 事件跟踪需求（Event Requirements）

本文档定义 Agent 运行过程中的事件跟踪需求，用于日志、审计、指标与可观测性。核心目标：**可跟踪模型执行的提示、工具调用前/后、以及工具的输入与输出**。

---

## 1. 概述

- **事件总线**：通过 `events.Bus` 发布/订阅，所有事件均带 `RequestID` 便于按请求关联。
- **触发位置**：Agent Run 生命周期由 `agent` 包发布 Run/Model 相关事件；工具执行由 `tool.Registry`（及可选 Agent）发布工具相关事件。
- **Payload**：统一使用 `map[string]any`，字段见下各事件说明；可选字段在实现时按需填充。

---

## 2. 事件类型与约定

### 2.1 Run 生命周期（已有）

| Kind | 触发时机 | Payload 建议 | 实现状态 |
|------|----------|--------------|----------|
| `agent.run.started` | 一次 Run 开始，在调用模型或工具之前 | `message_count`（本次请求消息条数） | ✅ 已实现 |
| `agent.run.completed` | Run 正常结束 | `text_length`（最终回答长度），可扩展 `step` 等 | ✅ 已实现 |
| `agent.run.error` | Run 过程中发生错误 | `error`（错误信息），可选 `step` | ✅ 已实现 |

### 2.2 模型执行跟踪（需完善）

目标：**跟踪“发给模型的提示”以及模型返回**。

| Kind | 触发时机 | Payload 建议 | 实现状态 |
|------|----------|--------------|----------|
| **模型调用前** | 每次调用模型 API 之前（如 `Chat` / `ChatWithTools`） | `message_count`、可选 `step`（ReAct 步数）、可选 `prompt_summary`（如仅长度或条数，避免落库完整 prompt） | ✅ 已实现（`agent.model.invoked`） |
| `agent.model.responded` | 模型 API 返回之后 | `text_length`，可扩展 `token_input` / `token_output` | ✅ 已实现（ChatAgent 与 ReAct 每步均发） |

**需求要点**：

- **模型执行的提示**：在「模型调用前」事件中体现。Payload 至少包含本次请求的 `message_count`（及 ReAct 下的 `step`）；若需记录更多，可使用 `prompt_summary`（例如 `{"message_count": n, "total_content_length": m}`），**不要求**在事件中落库完整 messages 内容，以兼顾审计与隐私。
- 若实现 `agent.model.invoked`，应在每次 `model.Chat` / `model.ChatWithTools` 调用前由 agent 发布，且 RequestID 与当前 Run 一致。

### 2.3 工具执行跟踪（需完善）

目标：**工具使用前、工具使用后、以及工具的输入和输出**。

| Kind | 触发时机 | Payload 建议 | 实现状态 |
|------|----------|--------------|----------|
| **工具调用前** | 在真正执行 `Execute(ctx, params)` 之前 | `tool`（工具名）、`input`（即本次调用的 `params`，即工具输入） | ✅ 已实现（`agent.tool.invoked`） |
| `agent.tool.executed` | 工具 `Execute` 返回之后（无论成功或失败） | `tool`、`input`（同上）、`output`（即 `Execute` 的返回值）、失败时 `error` | ✅ 已实现（含 input/output） |

**需求要点**：

- **工具使用前**：在 `tool.Registry` 的包装逻辑中，在调用 `orig(ctx, params)` **之前**发布「工具调用前」事件，Payload 包含 `tool` 与 `input`（即 `params`）。
- **工具使用后**：保持现有 `ToolExecuted` 在 `Execute` 返回后发布；**必须**在 Payload 中增加：
  - **input**：本次调用的参数（与「工具调用前」一致，便于前后关联）。
  - **output**：工具返回值（可 JSON 序列化或转为 `map[string]any`；若过大可仅记录类型或长度，由实现策略决定）。
- 敏感数据：若 `input`/`output` 含敏感信息，可在发布前脱敏或由监听方过滤，需求上不强制落库明文。

### 2.4 其他事件（保留）

- `agent.init`、`dataquery.write.proposed`、`dataquery.write.executed` 等按现有或业务需求使用，本需求不修改其定义。

---

## 3. 通用约定

- **RequestID**：所有事件均带 `RequestID`，与当前请求上下文一致（如从 `context` 的 `ContextKeyRequestID` 读取），便于一次 Run 内多事件关联。
- **Payload 结构**：统一 `map[string]any`；字段名建议小写、下划线风格（如 `message_count`、`text_length`）；可选字段可不传或省略。
- **顺序**：同一 Run 内，事件顺序应反映真实执行顺序（RunStarted → [ModelInvoked → ModelResponded] × N → [ToolInvoked → ToolExecuted] × M → RunCompleted / RunError）。

---

## 4. 与现有实现的对应关系

| 需求项 | 实现位置 | 状态 |
|--------|----------|------|
| Run 开始/结束/错误 | `agent/react_agent.go`、`agent/agent.go` | ✅ 已实现 |
| 模型调用前（提示） | `agent/agent.go`、`agent/react_agent.go` 在每次 `Chat`/`ChatWithTools` 前发 `ModelInvoked`，payload: message_count、step（ReAct） | ✅ 已实现 |
| 模型调用后 | `agent/agent.go`、`agent/react_agent.go` 每次模型返回后发 `ModelResponded`，payload: text_length、step（ReAct） | ✅ 已实现 |
| 工具调用前 | `tool/tool.go` 在 `Execute` 包装内、调用 `orig` 前发布 `ToolInvoked`，payload: tool, input | ✅ 已实现 |
| 工具调用后 + 输入输出 | `tool/tool.go` 的 `ToolExecuted` payload 含 tool、input、output、失败时 error | ✅ 已实现 |

---

## 5. 小结

- **模型执行**：通过「模型调用前」事件跟踪发给模型的提示（至少 message_count/step，可选 prompt_summary）；通过已有/扩展的 ModelResponded 跟踪模型输出。
- **工具执行**：通过「工具调用前」事件跟踪工具名与输入；通过扩展后的 ToolExecuted 跟踪工具名、输入与输出（及错误）。
- 实现时以本文档的 Payload 约定为准，并在 `events/event.go` 中补充新增的 Kind（如 `agent.model.invoked`、`agent.tool.invoked`）后，在 agent 与 tool 层按上表补齐发布逻辑。
