package agent

import (
	"context"

	"github.com/sath/memory"
	"github.com/sath/model"
	"github.com/sath/tool"
)

// ToolCallingModel 抽象出具备 tools API 能力的模型（当前由 OpenAIClient 实现）。
type ToolCallingModel interface {
	model.Model
	ChatWithTools(ctx context.Context, messages []model.Message, reg *tool.Registry, opts ...model.Option) (*model.Generation, error)
}

// ReActAgent 基于 tools API 的 ReAct 循环：
// 每一步由模型决定是否调用工具（通过 ToolCalls），底层模型负责调用本地工具并返回 observation，
// Agent 再把 observation 注入对话，让模型给出下一步推理或最终答案。
type ReActAgent struct {
	model    model.Model
	mem      memory.Memory
	tools    *tool.Registry
	maxSteps int
}

type ReActConfig struct {
	MaxSteps   int
	MaxHistory int
}

type ReActOption func(*ReActConfig)

func WithReActMaxSteps(n int) ReActOption {
	return func(c *ReActConfig) {
		if n > 0 {
			c.MaxSteps = n
		}
	}
}

func WithReActMaxHistory(n int) ReActOption {
	return func(c *ReActConfig) {
		if n > 0 {
			c.MaxHistory = n
		}
	}
}

// NewReActAgent 创建一个 ReAct 风格的 Agent。
// tools 允许为 nil，此时仅退化为普通对话 Agent。
func NewReActAgent(m model.Model, mem memory.Memory, tools *tool.Registry, opts ...ReActOption) *ReActAgent {
	cfg := ReActConfig{
		MaxSteps:   3,
		MaxHistory: 10,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &ReActAgent{
		model:    m,
		mem:      mem,
		tools:    tools,
		maxSteps: cfg.MaxSteps,
	}
}

func (a *ReActAgent) Run(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, nil
	}

	// 读取历史对话
	history, _ := a.mem.GetRecent(ctx, 10)
	var messages []model.Message
	for _, h := range history {
		messages = append(messages, h.Message)
	}
	messages = append(messages, req.Messages...)

	// 无工具或模型不支持 tools API 时退化为普通对话。
	tm, ok := a.model.(ToolCallingModel)
	if !ok || a.tools == nil {
		gen, err := a.model.Chat(ctx, messages)
		if err != nil {
			return nil, err
		}
		_ = a.mem.Add(ctx, memory.Entry{
			Message: model.Message{Role: "assistant", Content: gen.Text},
		})
		return &Response{Text: gen.Text}, nil
	}

	// 基于 tools API 的 ReAct 循环：
	// 每一步：
	//   1. 调用 ChatWithTools 让模型决定是否/如何调用工具；
	//      - 若未调用工具（ToolStep.Used=false），则 Text 视为最终答案，提前结束。
	//      - 若调用了工具，则 Text 为 observation（工具结果）。
	//   2. 将 observation 作为 tool 消息注入；
	//   3. 调用 Chat 让模型在 observation 基础上给出新的回复；
	//   4. 将新的回复作为 assistant 消息追加，并作为候选最终答案，为下一轮提供上下文。
	var lastAnswer string
	for step := 0; step < a.maxSteps; step++ {
		toolGen, err := tm.ChatWithTools(ctx, messages, a.tools)
		if err != nil {
			return nil, err
		}

		stepInfo, _ := toolGen.Raw.(model.ToolStep)
		// 未使用工具：认为模型已经有足够信息给出答案，进行一次「总结/最终回答」调用后结束。
		if !stepInfo.Used {
			// 将当前输出作为 assistant 消息写入上下文。
			messages = append(messages, model.Message{
				Role:    "assistant",
				Content: toolGen.Text,
			})
			// 追加一个明确的最终回答指令，避免把中间思考当成答案。
			messages = append(messages, model.Message{
				Role:    "user",
				Content: "请基于以上所有对话和工具结果，给出简洁明确的最终答案。如果已经给出了答案，则请复述该答案。",
			})

			finalGen, err := a.model.Chat(ctx, messages)
			if err != nil {
				return nil, err
			}
			lastAnswer = finalGen.Text
			break
		}

		// 将工具执行结果作为 tool 消息注入上下文。
		messages = append(messages, model.Message{
			Role:    "tool",
			Content: toolGen.Text,
		})

		finalGen, err := a.model.Chat(ctx, messages)
		if err != nil {
			return nil, err
		}
		lastAnswer = finalGen.Text

		// 将模型的回复加入对话，为下一轮提供上下文。
		messages = append(messages, model.Message{
			Role:    "assistant",
			Content: finalGen.Text,
		})
	}

	if lastAnswer == "" {
		return &Response{Text: ""}, nil
	}
	_ = a.mem.Add(ctx, memory.Entry{
		Message: model.Message{Role: "assistant", Content: lastAnswer},
	})
	return &Response{Text: lastAnswer}, nil
}
