package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"github.com/sath/tool"
)

// Test ChatWithTools 完整路径：构造请求 -> 模型返回 tool_call -> 本地工具执行 -> 返回结果 JSON 文本。
func TestOpenAIClient_ChatWithTools_ToolCallFlow(t *testing.T) {
	// 构造一个假的 OpenAI /chat/completions 端点，返回包含 tool_calls 的响应。
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		// 返回一个要求调用 calculator_add 的 tool_call
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleAssistant,
						Content: "",
						ToolCalls: []openai.ToolCall{
							{
								ID:   "call_1",
								Type: openai.ToolTypeFunction,
								Function: openai.FunctionCall{
									Name:      "calculator_add",
									Arguments: `{"a":1.5,"b":2.5}`,
								},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response error: %v", err)
		}
	}))
	defer ts.Close()

	cfg := openai.DefaultConfig("test")
	cfg.BaseURL = ts.URL
	cfg.HTTPClient = ts.Client()
	client := &OpenAIClient{
		apiKey: "test",
		client: openai.NewClientWithConfig(cfg),
		model:  "fake-model",
	}

	reg := tool.NewRegistry()
	if err := tool.RegisterCalculatorTool(reg); err != nil {
		t.Fatalf("register calculator tool error: %v", err)
	}

	msgs := []Message{
		{Role: "user", Content: "1.5 + 2.5 等于几？"},
	}

	gen, err := client.ChatWithTools(context.Background(), msgs, reg)
	if err != nil {
		t.Fatalf("ChatWithTools error: %v", err)
	}
	if gen == nil || gen.Text == "" {
		t.Fatalf("unexpected generation: %#v", gen)
	}

	var v float64
	if err := json.Unmarshal([]byte(gen.Text), &v); err != nil {
		t.Fatalf("unmarshal generation text error: %v (text=%q)", err, gen.Text)
	}
	if v != 4.0 {
		t.Fatalf("expected 4.0, got %v", v)
	}
}
