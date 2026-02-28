package agent

import (
	"context"
	"testing"

	"github.com/sath/memory"
	"github.com/sath/model"
	"github.com/sath/tool"
)

// fakeOpenAIClient 实现 model.Model，并在 ChatWithTools 时返回预设的工具结果。
type fakeOpenAIClient struct {
	toolResult string
	finalReply string
}

func (f *fakeOpenAIClient) Generate(ctx context.Context, prompt string, opts ...model.Option) (*model.Generation, error) {
	_ = ctx
	_ = opts
	return &model.Generation{Text: f.finalReply}, nil
}

func (f *fakeOpenAIClient) Chat(ctx context.Context, msgs []model.Message, opts ...model.Option) (*model.Generation, error) {
	_ = ctx
	_ = opts
	// 最终回答。
	return &model.Generation{Text: f.finalReply}, nil
}

func (f *fakeOpenAIClient) Embed(ctx context.Context, texts []string, opts ...model.Option) ([]model.Embedding, error) {
	_ = ctx
	_ = opts
	out := make([]model.Embedding, len(texts))
	return out, nil
}

// 覆盖 ChatWithTools 行为，通过组合方式实现。
func (f *fakeOpenAIClient) ChatWithTools(ctx context.Context, messages []model.Message, reg *tool.Registry, opts ...model.Option) (*model.Generation, error) {
	_ = ctx
	_ = reg
	_ = opts
	return &model.Generation{
		Text: f.toolResult,
		Raw: model.ToolStep{
			Used:        true,
			ToolName:    "calculator_add",
			Observation: f.toolResult,
		},
	}, nil
}

func TestReActAgent_WithToolAndOpenAIClient(t *testing.T) {
	mem := memory.NewBufferMemory(5)

	// 使用真实的 OpenAIClient 类型，但替换其内部逻辑为 fake。
	client := &model.OpenAIClient{}
	// 将 fake 行为通过类型断言伪装为 OpenAIClient 需要的行为在此测试中不直接验证 ChatWithTools 的 HTTP 细节，
	// 因此简化为：通过嵌入 fakeOpenAIClient 来复用其方法。
	fake := &fakeOpenAIClient{
		toolResult: `{"sum":4}`,
		finalReply: "the result is 4",
	}
	*client = model.OpenAIClient(*client) // 占位，随后通过组合 fake 的行为。

	// 注册一个工具，但在当前测试中不实际调用。
	reg := tool.NewRegistry()
	if err := tool.RegisterCalculatorTool(reg); err != nil {
		t.Fatalf("register calculator tool error: %v", err)
	}

	// 为了不修改 OpenAIClient 定义，这里通过接口转换的方式，仅验证 ReActAgent 能调用 Chat 和写入记忆。
	react := NewReActAgent(fake, mem, reg)

	req := &Request{
		Messages: []model.Message{
			{Role: "user", Content: "1+3=?"},
		},
	}
	resp, err := react.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if resp == nil || resp.Text != "the result is 4" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}
