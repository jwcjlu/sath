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
	// ModelResponded 表示已收到模型响应（Chat/Generate 返回）。
	ModelResponded Kind = "agent.model.responded"
	// ToolExecuted 表示执行了一次工具调用。
	ToolExecuted Kind = "agent.tool.executed"
	// RunCompleted 表示一次 Run 正常完成。
	RunCompleted Kind = "agent.run.completed"
	// RunError 表示 Run 过程中发生错误。
	RunError Kind = "agent.run.error"

	// DataQueryIntent 表示数据对话意图识别完成。
	DataQueryIntent Kind = "dataquery.intent"
	// DataQueryExecuted 表示数据对话 DSL 已执行。
	DataQueryExecuted Kind = "dataquery.executed"
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
