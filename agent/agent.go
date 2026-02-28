package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/sath/events"
	"github.com/sath/memory"
	"github.com/sath/model"
)

// Request 表示对 Agent 的一次请求，V0.1 仅关注文本消息。
type Request struct {
	Messages  []model.Message
	Metadata  map[string]any
	RequestID string // 可选，用于事件关联与审计；为空时 Run 内可自动生成。
}

// Response 表示 Agent 的一次回复。
type Response struct {
	Text     string
	Metadata map[string]any
}

// Agent 定义统一的 Agent 接口。
type Agent interface {
	Run(ctx context.Context, req *Request) (*Response, error)
}

// ChatAgent 是 V0.1 的默认对话 Agent 实现。
type ChatAgent struct {
	model  model.Model
	mem    memory.Memory
	config ChatConfig
}

// ChatConfig 控制 ChatAgent 的行为。
type ChatConfig struct {
	MaxHistory int
	EventBus   *events.Bus // 可选；非空时在生命周期关键点发布事件。
}

// Option 为 ChatAgent 提供可选配置。
type Option func(*ChatConfig)

func WithMaxHistory(n int) Option {
	return func(c *ChatConfig) {
		if n > 0 {
			c.MaxHistory = n
		}
	}
}

// WithEventBus 设置用于发布生命周期事件的总线；为 nil 时不发布事件。
func WithEventBus(bus *events.Bus) Option {
	return func(c *ChatConfig) {
		c.EventBus = bus
	}
}

// NewChatAgent 创建一个默认对话 Agent。
func NewChatAgent(m model.Model, mem memory.Memory, opts ...Option) *ChatAgent {
	cfg := ChatConfig{
		MaxHistory: 10,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &ChatAgent{
		model:  m,
		mem:    mem,
		config: cfg,
	}
}

func requestID(req *Request) string {
	if req != nil && req.RequestID != "" {
		return req.RequestID
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return ""
}

// Run 将历史记忆与当前请求合并后调用底层模型，并在配置了 EventBus 时发布生命周期事件。
func (a *ChatAgent) Run(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, nil
	}

	rid := requestID(req)
	bus := a.config.EventBus
	if bus == nil {
		bus = events.DefaultBus()
	}

	emit := func(kind events.Kind, payload map[string]any) {
		if bus == nil {
			return
		}
		if payload == nil {
			payload = make(map[string]any)
		}
		bus.Publish(ctx, events.Event{Kind: kind, Payload: payload, RequestID: rid})
	}

	emit(events.RunStarted, map[string]any{"message_count": len(req.Messages)})

	history, _ := a.mem.GetRecent(ctx, a.config.MaxHistory)
	var messages []model.Message
	for _, h := range history {
		messages = append(messages, h.Message)
	}
	messages = append(messages, req.Messages...)

	gen, err := a.model.Chat(ctx, messages)
	if err != nil {
		emit(events.RunError, map[string]any{"error": err.Error()})
		return nil, err
	}

	emit(events.ModelResponded, map[string]any{"text_length": len(gen.Text)})

	_ = a.mem.Add(ctx, memory.Entry{
		Message: model.Message{
			Role:    "assistant",
			Content: gen.Text,
		},
	})

	emit(events.RunCompleted, map[string]any{"text_length": len(gen.Text)})
	resp := &Response{Text: gen.Text}
	if gen.TokenUsage != nil {
		if resp.Metadata == nil {
			resp.Metadata = make(map[string]any)
		}
		resp.Metadata["token_input"] = gen.TokenUsage.InputTokens
		resp.Metadata["token_output"] = gen.TokenUsage.OutputTokens
	}
	return resp, nil
}
