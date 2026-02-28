package agent

import (
	"context"

	"github.com/sath/model"
)

// PlanExecuteAgent 将复杂任务拆解为规划 + 执行两个阶段。
// 当前版本：使用 planner 的 Generate 得到自然语言计划（不强制结构化），
// 然后将原始请求交给 worker Agent 执行，并在需要时可将计划写入元数据。
type PlanExecuteAgent struct {
	planner model.Model
	worker  Agent
}

func NewPlanExecuteAgent(planner model.Model, worker Agent) *PlanExecuteAgent {
	return &PlanExecuteAgent{
		planner: planner,
		worker:  worker,
	}
}

func (p *PlanExecuteAgent) Run(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, nil
	}

	// 规划阶段：将用户输入合并为一个 prompt。
	prompt := ""
	for _, m := range req.Messages {
		if m.Role == "user" {
			if prompt != "" {
				prompt += "\n"
			}
			prompt += m.Content
		}
	}
	if prompt == "" && len(req.Messages) > 0 {
		prompt = req.Messages[len(req.Messages)-1].Content
	}

	planGen, err := p.planner.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// 将计划写入 Metadata，传递给执行阶段（如有下游需要可使用）。
	if req.Metadata == nil {
		req.Metadata = make(map[string]any)
	}
	req.Metadata["plan"] = planGen.Text

	// 执行阶段：交给 worker Agent 按计划完成任务。
	return p.worker.Run(ctx, req)
}
