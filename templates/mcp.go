package templates

import (
	"context"
	"strings"

	"github.com/sath/agent"
	"github.com/sath/events"
	"github.com/sath/memory"
	"github.com/sath/middleware"
	"github.com/sath/model"
	"github.com/sath/tool"
)

// Config 描述 MCP Agent 的核心配置。
type Config struct {
	// MCP 服务端点（HTTP/SSE 等），由调用方保证连通性。
	Endpoint string
	// MCP 服务 ID，用于在日志/指标中区分不同服务。
	ID string

	// ReAct 最大步骤数，<=0 时默认为 3。
	MaxReActSteps int
	// 会话短期记忆窗口大小，<=0 时默认为 10。
	MaxHistory int
	Backend    string
}

// BuildMCPSystemPrompt 构造 MCP Agent 的系统级提示。
// 说明其职责、工作流以及如何使用底层 MCP 工具。
func BuildMCPSystemPrompt() string {
	var b strings.Builder
	b.WriteString("你是一个 MCP 工具调用助手，负责代表调用方安全、可靠地调用外部 MCP 服务提供的工具能力。\n\n")
	b.WriteString("【总体原则】\n")
	b.WriteString("1. 你不能编造或猜测 MCP 工具的返回结果，只能依据实际调用结果回答；若调用失败，应如实说明原因。\n")
	b.WriteString("2. 当不需要外部能力时，可以直接基于已有上下文回答；只有在明确需要依赖外部系统数据或能力时，才调用 MCP 工具。\n")
	b.WriteString("3. 对用户可见的回答应是自然语言总结，不暴露底层协议细节。\n\n")

	b.WriteString("【推荐工作流（ReAct 思考→行动→观察）】\n")
	b.WriteString("1. 理解调用方问题或意图，在思考中判断是否需要调用 MCP 工具。\n")
	b.WriteString("2. 若需要调用：根据工具描述选择合适的工具，并构造合理的参数；使用工具一次只解决一个子问题。\n")
	b.WriteString("3. 根据工具返回结果更新你的理解，必要时继续调用其他工具，直到可以给出清晰的最终结论。\n")
	b.WriteString("4. 最终回答时，用自然语言概括关键信息，不简单回显原始 JSON。\n")

	return b.String()
}

// NewMCPAgentHandler 构建一个基于 MCP 工具的 ReAct Agent 处理器。
// 该 Handler 可作为独立 HTTP 路由或被其他 Agent 作为“子 Agent”调用。
func NewMCPAgentHandler(m model.Model, mem memory.Memory, cfg Config, mws ...middleware.Middleware) middleware.Handler {
	if cfg.MaxReActSteps <= 0 {
		cfg.MaxReActSteps = 20
	}
	if cfg.MaxHistory <= 0 {
		cfg.MaxHistory = 10
	}

	// 注册 MCP 远端工具为本地 tools。
	reg := tool.NewRegistry()
	if bus := events.DefaultBus(); bus != nil {
		reg.SetEventBus(bus)
	}
	tool.RegisterMcpTool(reg, &tool.McpConfig{
		Endpoint: cfg.Endpoint,
		Id:       cfg.ID,
		Backend:  cfg.Backend,
	})

	reactOpts := []agent.ReActOption{
		agent.WithReActMaxSteps(cfg.MaxReActSteps),
		agent.WithReActMaxHistory(cfg.MaxHistory),
	}
	if bus := events.DefaultBus(); bus != nil {
		reactOpts = append(reactOpts, agent.WithReActEventBus(bus))
	}
	react := agent.NewReActAgent(m, mem, reg, reactOpts...)

	sys := BuildMCPSystemPrompt()

	final := func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		if req == nil {
			return nil, nil
		}
		// 在请求消息前注入 MCP Agent 的系统提示。
		llmReq := *req
		llmReq.Messages = append([]model.Message{
			{Role: "system", Content: sys},
		}, req.Messages...)
		if llmReq.RequestID != "" {
			ctx = context.WithValue(ctx, tool.ContextKeyRequestID, llmReq.RequestID)
		}
		return react.Run(ctx, &llmReq)
	}

	return middleware.Chain(final, mws...)
}
