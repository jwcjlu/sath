# 扩展框架：插件与事件钩子

本文说明如何通过**插件系统**与**事件钩子**扩展 sath，包括注册模型、工具、中间件与生命周期监听（F6.1–F6.4）。

## 1. 扩展方式概览

| 扩展点 | 方式 | 说明 |
|--------|------|------|
| 模型 | `model.RegisterProvider` / `plugin.RegisterModelProvider` | 使 `model.NewFromIdentifier("your/provider")` 可用 |
| 工具 | `plugin.RegisterTool` | 应用通过 `plugin.RegisteredTools()` 合并到 `tool.Registry` |
| 中间件 | `plugin.RegisterMiddleware` | 应用通过 `plugin.RegisteredMiddlewares()` 按名组装链 |
| 生命周期 | `events.Bus` + `plugin.RegisterEventListener` | 在 Agent 关键点发布/订阅事件 |

**当前阶段不支持运行时热插拔**：所有插件应在进程启动前完成注册（通常通过 `import _ "your/plugin"` 在 `init()` 中注册）。

---

## 2. 插件注册中心（plugin 包）

插件通过匿名导入触发 `init()`，在 `plugin.Registry` 中注册组件：

```go
import _ "your/module/plugins/your_plugin"
```

### 2.1 注册模型 Provider

使 `model.NewFromIdentifier("myprovider/model-name")` 使用你的实现：

```go
// your_plugin/init.go
package your_plugin

import (
	"github.com/sath/model"
	"github.com/sath/plugin"
)

func init() {
	plugin.RegisterModelProvider("myprovider", func(id string) (model.Model, error) {
		// 解析 id，创建并返回你的 model.Model 实现
		return NewMyModel(id), nil
	})
}
```

`model.NewFromIdentifier` 会先查找已注册的 Provider，再回退到内置的 `openai` / `ollama`。

### 2.2 注册工具

```go
func init() {
	plugin.RegisterTool(tool.Tool{
		Name:        "my_tool",
		Description: "描述给模型看的用途",
		Parameters:  map[string]any{...},
		Execute:     func(ctx context.Context, params map[string]any) (any, error) { ... },
	})
}
```

主程序可将插件工具合并到现有 `tool.Registry`：

```go
reg := tool.NewRegistry()
for _, t := range plugin.RegisteredTools() {
	_ = reg.Register(t)
}
```

### 2.3 注册中间件

```go
func init() {
	plugin.RegisterMiddleware("my_middleware", func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
			// 前置逻辑
			return next(ctx, req)
		}
	})
}
```

主程序按名取用并组装链（例如根据配置选择中间件）。

### 2.4 注册事件监听器

监听器会收到 Agent 生命周期事件（见下文「事件系统」）。若主程序设置了默认总线，插件可订阅：

```go
func init() {
	plugin.RegisterEventListener(true, func(ctx context.Context, e events.Event) {
		// 异步处理，例如打日志、审计、通知
		log.Printf("event: %s request_id=%s", e.Kind, e.RequestID)
	})
}
```

主程序在设置默认总线后，可一次性将已注册监听器挂载到该总线：

```go
bus := events.NewBus()
events.SetDefaultBus(bus)
plugin.ApplyListenersTo(bus)
```

---

## 3. 事件系统（events 包）

### 3.1 事件类型

| Kind | 说明 |
|------|------|
| `AgentInit` | Agent/组件初始化完成 |
| `RunStarted` | 一次 `Run` 开始 |
| `ModelResponded` | 已收到模型响应（Chat/Generate 返回） |
| `ToolExecuted` | 执行了一次工具调用 |
| `RunCompleted` | 一次 `Run` 正常完成 |
| `RunError` | `Run` 过程中发生错误 |

`Event` 结构包含 `Kind`、`Payload`（map）、`RequestID`、`At`（时间），便于日志、审计与指标。

### 3.2 总线与订阅

- **同步订阅**：`bus.Subscribe(false, listener)`，在 `Publish` 时顺序执行，会阻塞发布方。
- **异步订阅**：`bus.Subscribe(true, listener)`，在独立 goroutine 中执行，不阻塞发布方。

```go
bus := events.NewBus()
bus.Subscribe(false, func(ctx context.Context, e events.Event) {
	// 同步：日志、校验等
})
bus.Subscribe(true, func(ctx context.Context, e events.Event) {
	// 异步：上报指标、通知等
})
```

### 3.3 在 Agent 中发布事件

`ChatAgent` 支持通过 `WithEventBus(bus)` 注入总线；若未注入且 `events.DefaultBus()` 非空，则使用默认总线。在 `Run` 中会在以下时点发布事件：

- 开始：`RunStarted`
- 模型返回后：`ModelResponded`
- 成功结束：`RunCompleted`
- 发生错误：`RunError`

请求可设置 `Request.RequestID` 以便在事件中关联同一次调用。

---

## 4. 自定义组件（F6.2）

除插件注册外，所有核心能力均通过接口与注入扩展：

- **模型**：实现 `model.Model`，或通过 `model.RegisterProvider` 接入工厂。
- **记忆**：实现 `memory.Memory`（如 `BufferMemory`、向量记忆等）。
- **工具**：实现 `tool.Tool` 并注册到 `tool.Registry` 或通过插件注册。
- **中间件**：实现 `middleware.Middleware`，用 `middleware.Chain` 组装。
- **Agent**：实现 `agent.Agent`，或使用 `ChatAgent` / `ReActAgent` 等并注入上述组件。

无需修改框架源码即可替换或扩展任一环节。

---

## 5. 插件打包与约束

- 插件应为独立包，在 `init()` 中调用 `plugin.Register*` 或 `model.RegisterProvider`。
- 主程序通过匿名导入加载插件：`import _ "your/plugin"`。
- 当前**不支持**从配置文件或运行时动态加载二进制插件；所有扩展为编译期绑定。

更多示例见仓库内各包测试（如 `plugin/registry_test.go`、`agent/agent_events_test.go`）。
