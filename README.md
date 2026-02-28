# sath AI Agent 框架（V0.1 MVP）

轻量级、可扩展的 Go 语言 AI Agent 开发框架原型，实现了最小可用的：

- 统一模型接口与 OpenAI 文本对话适配器
- 默认对话 Agent（短期记忆 BufferMemory）
- 中间件链（日志 + panic 恢复）
- 配置加载（环境变量）
- 工具注册与简单 Function Calling 示例

> 当前为 V0.1 MVP 骨架，主要目标是“能跑起来、能集成、易扩展”，高级特性（多厂商、多模态、RAG、完整可观察性、插件系统等）将在后续版本迭代。

---

## 快速开始（对话 Demo）

### 1. 前置条件

- Go 1.20+
- 有效的 OpenAI API Key

### 2. 克隆与构建

```bash
git clone <your-repo-url> sath
cd sath
go build ./...
```

### 3. 运行对话 Demo（REPL）

**方式一：使用 CLI（推荐）**

```bash
# 构建 CLI
go build -o sath ./cmd/sath

# Windows PowerShell
$env:OPENAI_API_KEY="your-key"
.\sath demo

# 或 Linux/macOS
export OPENAI_API_KEY="your-key"
./sath demo
```

**方式二：直接运行 demo 包**

```bash
cd cmd/demo

# Windows PowerShell
$env:OPENAI_API_KEY="your-key"

# 或 Linux/macOS
export OPENAI_API_KEY="your-key"

go run .
```

终端示例：

```text
AI Agent demo started. Type 'exit' to quit.
> 你好，你是谁？
...（模型回复）...
```

---

## 工具调用 Demo（Function Calling + 本地工具）

V0.1 提供了一个最小示例：通过 OpenAI Function Calling 调用本地注册的 `calculator_add` 工具完成加法运算。

### 运行示例

```bash
cd cmd/tool_demo

# 确保已经设置 OPENAI_API_KEY
go run .
```

预期输出类似：

```text
calculator_add result: 4
```

> 实际输出取决于模型是否选择调用工具以及 JSON 解析结果。

---

## 目录结构概览（V0.1）

```text
agent/        默认对话 Agent 接口与实现（ChatAgent）
model/        模型接口、OpenAI 适配器、Function Calling 桥接
memory/       会话短期记忆接口与 BufferMemory 实现
middleware/   中间件抽象与日志/恢复中间件
tool/         工具定义、注册表与示例工具（calculator_add）
config/       核心配置结构与环境变量加载
cmd/demo/     文本对话 REPL 示例
cmd/tool_demo/工具调用示例
```

---

## 设计原则（简要）

- **开箱即用**：仅配置 `OPENAI_API_KEY` 即可运行对话 Demo。
- **接口优先**：`Model` / `Agent` / `Memory` / `Tool` / `Middleware` 统一抽象，便于后续扩展。
- **可扩展性**：各组件通过接口与 Option 模式解耦，可替换实现（如自定义模型、记忆、工具、中间件）。
- **可观察性雏形**：结构化日志 + panic 恢复，为后续接入 Prometheus / OpenTelemetry 预留空间。

---

## CLI 工具（sath）

| 命令 | 说明 |
|------|------|
| `sath init` | 在当前目录初始化新项目骨架（main.go + config.yaml） |
| `sath demo` | 运行内置对话 Agent 示例（REPL） |
| `sath serve` | 启动 HTTP 服务，提供 `POST /chat` 与 `GET /metrics` |

示例：`sath init -d myapp` 在 `myapp/` 下生成骨架；`sath serve -a :8080` 监听 8080 端口。

---

## Roadmap 与版本策略

### Roadmap

在当前骨架基础上，后续版本计划逐步引入：

- 多模型与多厂商支持（OpenAI、Claude、通义、文心等）
- 多模态能力（文本 + 图片等）
- RAG 与长期向量记忆
- 完整可观察性（指标、追踪、健康检查）
- 插件系统与事件钩子

### 版本策略

本项目采用**语义化版本**（Semantic Versioning，如 `v0.1.0`、`v1.0.0`）：

- **主版本号**：不兼容的 API 或行为变更
- **次版本号**：向后兼容的功能新增
- **修订号**：向后兼容的问题修复

在达到 1.0 之前，0.x 版本可能仍有接口微调，但会尽量保持可迁移性。

---

## 社区与贡献

- [CONTRIBUTING.md](CONTRIBUTING.md)：贡献流程、分支与 PR 约定
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)：行为准则
- 提交 Bug 或功能建议请使用 [Issue 模板](.github/ISSUE_TEMPLATE/)

欢迎在此基础上继续演进，或根据你的业务场景扩展自定义组件。
