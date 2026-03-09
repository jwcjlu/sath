package tool

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/sath/events"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RegisterToolOptions 用于注册工具时的可选覆盖（如按数据源类型的描述文案）。
// 若 Description 非空，则覆盖该工具的默认 Description（模型可见）。
type RegisterToolOptions struct {
	Description string
}

// ExecuteFunc 是工具的执行函数签名。
type ExecuteFunc func(ctx context.Context, params map[string]any) (any, error)

// Tool 描述一个可被 Agent 调用的工具。
type Tool struct {
	Name        string
	Description string
	// Parameters 描述参数结构，遵循 JSON Schema 风格（V0.1 可简化为任意 map）。
	Parameters any
	Execute    ExecuteFunc
}

// ContextKeyRequestID 为从 context 中读取 request_id 的键，与 templates 中 WithValue("request_id", ...) 一致。
const ContextKeyRequestID = "request_id"

var tracer = otel.Tracer("github.com/sath/tool")

// Registry 维护一组可用工具。
type Registry struct {
	mu           sync.RWMutex
	tools        map[string]Tool
	mcpServerIDs map[string]struct{} // 已注册的 MCP 服务 ID，用于按 Skill 使用时幂等注册
	eventBus     *events.Bus         // 可选；非空时在每次工具执行后发布 ToolExecuted
}

// NewRegistry 创建一个空的工具注册表。
func NewRegistry() *Registry {
	return &Registry{
		tools:        make(map[string]Tool),
		mcpServerIDs: make(map[string]struct{}),
	}
}

// HasMcpServer 返回该 MCP 服务 ID 是否已在本 Registry 中注册过（用于按 Skill 动态注册时去重）。
func (r *Registry) HasMcpServer(id string) bool {
	if id == "" {
		return false
	}
	r.mu.RLock()
	_, ok := r.mcpServerIDs[id]
	r.mu.RUnlock()
	return ok
}

// MarkMcpServer 标记某 MCP 服务 ID 已在本 Registry 注册，调用方应在成功注册 MCP 工具后调用。
func (r *Registry) MarkMcpServer(id string) {
	if id == "" {
		return
	}
	r.mu.Lock()
	if r.mcpServerIDs == nil {
		r.mcpServerIDs = make(map[string]struct{})
	}
	r.mcpServerIDs[id] = struct{}{}
	r.mu.Unlock()
}

// SetEventBus 设置事件总线；非空时，工具执行前发布 events.ToolInvoked、执行后发布 events.ToolExecuted（RequestID 从 context 的 ContextKeyRequestID 读取）。
func (r *Registry) SetEventBus(b *events.Bus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventBus = b
}

// Register 向注册表中添加一个工具。若已设置 EventBus，则包装 Execute：执行前发布 ToolInvoked，执行后发布 ToolExecuted（含 input/output）。
func (r *Registry) Register(t Tool) error {
	if t.Name == "" {
		return errors.New("tool name is empty")
	}
	if t.Execute == nil {
		return errors.New("tool execute is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name]; exists {
		return errors.New("tool already registered: " + t.Name)
	}
	bus := r.eventBus
	name := t.Name
	orig := t.Execute
	if bus != nil {
		t.Execute = func(ctx context.Context, params map[string]any) (any, error) {
			// 为每次工具调用创建 trace span，便于调用链追踪。
			ctx, span := tracer.Start(ctx, "tool."+name, trace.WithSpanKind(trace.SpanKindInternal))
			span.SetAttributes(attribute.String("tool.name", name))
			rid, _ := ctx.Value(ContextKeyRequestID).(string)
			if rid != "" {
				span.SetAttributes(attribute.String("request.id", rid))
			}

			input := any(params)
			if params == nil {
				input = map[string]any{}
			}
			span.SetAttributes(attribute.String("tool.input.type", fmt.Sprintf("%T", input)))

			bus.Publish(ctx, events.Event{
				Kind:      events.ToolInvoked,
				RequestID: rid,
				Payload:   map[string]any{"tool": name, "input": input},
			})

			result, err := orig(ctx, params)

			span.SetAttributes(attribute.String("tool.output.type", fmt.Sprintf("%T", result)))
			if err != nil {
				span.RecordError(err)
				span.SetAttributes(attribute.Bool("tool.error", true))
			}
			defer span.End()

			payload := map[string]any{"tool": name, "input": input, "output": result}
			if err != nil {
				payload["error"] = err.Error()
			}
			bus.Publish(ctx, events.Event{Kind: events.ToolExecuted, RequestID: rid, Payload: payload})
			return result, err
		}
	}
	r.tools[t.Name] = t
	return nil
}

// Get 根据名称获取工具。
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List 返回当前注册的所有工具。
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}
