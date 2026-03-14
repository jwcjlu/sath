package templates

import (
	"context"
	"fmt"
	"strings"

	"github.com/sath/agent"
	"github.com/sath/config"
	"github.com/sath/events"
	"github.com/sath/memory"
	"github.com/sath/middleware"
	"github.com/sath/model"
	"github.com/sath/skills"
	"github.com/sath/tool"
)

// BuildSkillsSummary 根据 Skill 列表构造一段简短的系统提示摘要。
// maxCount <= 0 时默认展示最多 8 个 Skill。
func BuildSkillsSummary(all []skills.SkillMeta, maxCount int) string {
	if len(all) == 0 {
		return ""
	}
	if maxCount <= 0 {
		maxCount = 8
	}
	if len(all) > maxCount {
		all = all[:maxCount]
	}
	var b strings.Builder
	b.WriteString("【可用 Skills（按需加载）】\n")
	b.WriteString("你可以按需加载以下技能以增强能力：\n")
	b.WriteString("- 使用 `load_skill` / `read_skill_file` / `execute_skill_script` 这三个工具与 Skills 交互。\n")
	b.WriteString("- 列表中的 **Skill 名称只是参数，不是工具名称**，不要尝试直接调用它们作为工具。\n")
	b.WriteString("当你需要使用某个 Skill 时，请先调用 `load_skill`，在阅读完整 SKILL.md 后，再按其中说明调用其它工具或继续推理。\n")
	for _, m := range all {
		desc := strings.TrimSpace(m.Description)
		if desc == "" {
			desc = "(无描述)"
		}
		b.WriteString(fmt.Sprintf("- %s：%s\n", m.Name, desc))
	}
	return b.String()
}

// buildSkillsAwareSystemPrompt 构造一个带 Skills 摘要的对话 Agent 系统提示。
func buildSkillsAwareSystemPrompt(skillsIdx *skills.Index) string {
	var b strings.Builder
	b.WriteString("你是一个具备 Skills 能力的通用对话助手。\n")
	b.WriteString("Skills 以 SKILL.md 文件的形式提供特定领域或任务的操作手册。\n")
	b.WriteString("注意：Skill 的名称（如 mysql-employees-analysis）不是工具名，不能直接当作工具调用；你只能调用显式提供的工具，比如 `load_skill` / `read_skill_file` / `execute_skill_script`。\n")
	b.WriteString("当你判断与某个 Skill 高度相关时，应先调用 `load_skill(name)` 获取该 Skill 的完整内容，并严格遵循其中给出的工作流与工具使用说明。\n")
	b.WriteString("重要：当你缺少必要信息、无法访问外部系统或脚本执行被禁用时，不要凭空编造具体结果（例如版本号、数量、精确日志内容等）。此时应如实说明受限原因，可以给出一般性的排查建议，但必须明确这不是基于真实执行结果的结论。\n")
	b.WriteString("关于脚本执行：不要事先假定「脚本执行已禁用」。仅当你实际调用了 `execute_skill_script` 且工具明确返回了脚本被禁用的错误信息时，才向用户说明需要开启 skills.allow_script_execution；未调用工具前不得提前给出此类结论。\n\n")

	if skillsIdx != nil {
		summary := BuildSkillsSummary(skillsIdx.All(), 8)
		if summary != "" {
			b.WriteString(summary)
			b.WriteString("\n")
		}
	}

	b.WriteString("使用建议：当你判断某个任务与已知 Skill 高度相关时，应先调用 load_skill(name) 获取完整 Skill 内容，再依据其中的工作流规划后续工具调用或回答策略。\n")
	return b.String()
}

// NewSkillsAwareChatHandlerFromConfig 构建一个支持 Skills 的 ReAct 对话 Handler。
// 它使用 ReActAgent + load_skill 工具，并在 System Prompt 中注入 Skills 摘要。
func NewSkillsAwareChatHandlerFromConfig(cfg config.Config, skillsIdx *skills.Index, middlewareByName map[string]middleware.Middleware) (middleware.Handler, error) {
	m, err := model.NewFromIdentifier(cfg.ModelName)
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	mem := memory.NewBufferMemory(cfg.MaxHistory)

	mws := make([]middleware.Middleware, 0, len(cfg.Middlewares)+2)
	mws = append(mws, middleware.RecoveryMiddleware, middleware.LoggingMiddleware)
	for _, name := range cfg.Middlewares {
		if mw, ok := middlewareByName[name]; ok {
			mws = append(mws, mw)
		}
	}

	final := func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		if req == nil {
			return nil, nil
		}

		reg := tool.NewRegistry()
		if bus := events.DefaultBus(); bus != nil {
			reg.SetEventBus(bus)
		}
		// MCP 不在请求开始时全量注册；仅在模型通过 load_skill 明确使用某 Skill 时，将该 Skill 声明的 MCP 注册到当前上下文。
		var mcpServers []tool.McpServerEntry
		for _, srv := range cfg.Skills.MCPServers {
			mcpServers = append(mcpServers, tool.McpServerEntry{
				Endpoint: srv.Endpoint,
				Id:       srv.ID,
				Backend:  srv.Backend,
			})
		}
		if skillsIdx != nil {
			_ = tool.RegisterLoadSkillTool(reg, skillsIdx, mcpServers)
			scriptOpts := &tool.ExecuteSkillScriptOptions{
				AllowedExtensions: cfg.Skills.ScriptAllowedExtensions,
				TimeoutSeconds:    cfg.Skills.ScriptTimeoutSeconds,
			}
			_ = tool.RegisterExecuteSkillScriptTool(reg, skillsIdx, cfg.Skills.AllowScriptExecution, scriptOpts)
		}

		reactOpts := []agent.ReActOption{agent.WithReActMaxSteps(20)}
		if bus := events.DefaultBus(); bus != nil {
			reactOpts = append(reactOpts, agent.WithReActEventBus(bus))
		}
		react := agent.NewReActAgent(m, mem, reg, reactOpts...)

		sys := buildSkillsAwareSystemPrompt(skillsIdx)
		llmReq := *req
		llmReq.Messages = append([]model.Message{
			{Role: "system", Content: sys},
		}, req.Messages...)

		if llmReq.Metadata == nil {
			llmReq.Metadata = make(map[string]any)
		}
		if _, ok := llmReq.Metadata["agent_name"]; !ok {
			llmReq.Metadata["agent_name"] = "chat-skills"
		}
		if llmReq.RequestID != "" {
			ctx = context.WithValue(ctx, tool.ContextKeyRequestID, llmReq.RequestID)
		}

		return react.Run(ctx, &llmReq)
	}

	return middleware.Chain(final, mws...), nil
}
