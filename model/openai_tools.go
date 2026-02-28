package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/sath/tool"
)

// ChatWithTools 使用 go-openai 的 tools API，结合本地 tool.Registry 执行工具，并返回最终结果。
// 当前简化：仅处理首个 tool_call，并将工具执行结果序列化为 JSON 文本返回。
func (c *OpenAIClient) ChatWithTools(ctx context.Context, messages []Message, reg *tool.Registry, opts ...Option) (*Generation, error) {
	if len(messages) == 0 {
		return nil, errors.New("messages is empty")
	}
	if reg == nil {
		return nil, errors.New("tool registry is nil")
	}

	callCfg := ApplyOptions(opts...)
	modelName := c.model
	if callCfg.ModelName != "" {
		modelName = callCfg.ModelName
	}

	tools := reg.List()
	if len(tools) == 0 {
		return nil, errors.New("no tools registered")
	}

	req := openai.ChatCompletionRequest{
		Model:       modelName,
		Messages:    make([]openai.ChatCompletionMessage, 0, len(messages)),
		Tools:       make([]openai.Tool, 0, len(tools)),
		ToolChoice:  "auto",
		Temperature: float32(callCfg.Temperature),
		MaxTokens:   callCfg.MaxTokens,
	}

	for _, m := range messages {
		req.Messages = append(req.Messages, openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	for _, tl := range tools {
		req.Tools = append(req.Tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tl.Name,
				Description: tl.Description,
				Parameters:  tl.Parameters,
			},
		})
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	msg := resp.Choices[0].Message
	// 如果没有 tool_calls，直接返回模型文本，并标记未使用工具。
	if len(msg.ToolCalls) == 0 {
		return &Generation{
			Text: msg.Content,
			Raw: ToolStep{
				Used:        false,
				ToolName:    "",
				Observation: nil,
			},
		}, nil
	}

	call := msg.ToolCalls[0]
	tl, ok := reg.Get(call.Function.Name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", call.Function.Name)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &params); err != nil {
		return nil, fmt.Errorf("unmarshal tool arguments: %w", err)
	}

	result, err := tl.Execute(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("execute tool %s: %w", tl.Name, err)
	}

	// 简化：直接将工具执行结果转成文本（JSON）返回。
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal tool result: %w", err)
	}

	return &Generation{
		Text: string(resultBytes),
		Raw: ToolStep{
			Used:        true,
			ToolName:    tl.Name,
			Observation: result,
		},
	}, nil
}
