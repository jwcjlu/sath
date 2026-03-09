package events

import (
	"context"
	"time"
)

// Kind 表示生命周期事件类型。
type Kind string

const (
	// AgentInit 表示 Agent 或组件初始化完成。
	AgentInit Kind = "agent.init"
	// RunStarted 表示一次 Run 开始。
	RunStarted Kind = "agent.run.started"
	// ModelInvoked 表示即将调用模型 API（Chat/ChatWithTools），用于跟踪发给模型的提示。
	ModelInvoked Kind = "agent.model.invoked"
	// ModelResponded 表示已收到模型响应（Chat/Generate 返回）。
	ModelResponded Kind = "agent.model.responded"
	// ToolInvoked 表示即将执行工具调用（Execute 调用前），Payload 含 tool、input。
	ToolInvoked Kind = "agent.tool.invoked"
	// ToolExecuted 表示执行了一次工具调用（Execute 返回后），Payload 含 tool、input、output、失败时 error。
	ToolExecuted Kind = "agent.tool.executed"
	// RunCompleted 表示一次 Run 正常完成。
	RunCompleted Kind = "agent.run.completed"
	// RunError 表示 Run 过程中发生错误。
	RunError Kind = "agent.run.error"

	// DataQueryWriteProposed 表示一次数据写/改已被提议（尚未执行），等待用户确认。
	DataQueryWriteProposed Kind = "dataquery.write.proposed"
	// DataQueryWriteExecuted 表示一次数据写/改已实际执行（成功或失败），用于审计。
	DataQueryWriteExecuted Kind = "dataquery.write.executed"
)

// Event 表示一条生命周期事件，用于日志、审计、指标等扩展。
type Event struct {
	Kind      Kind
	Payload   map[string]any
	RequestID string
	At        time.Time
}

// Listener 是事件监听器，可在 Subscribe 时指定同步或异步调用。
type Listener func(ctx context.Context, e Event)
