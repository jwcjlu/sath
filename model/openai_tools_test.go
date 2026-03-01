package model

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestOpenAIClient_ChatWithTools_ToolErrorAsObservation 验证：工具执行返回错误时，ChatWithTools 将错误作为观察结果返回（gen, nil），
// 而非 (nil, err)，以便 ReAct 可继续循环、模型根据错误决定下一步。
func TestOpenAIClient_ChatWithTools_ToolErrorAsObservation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
									Name:      "failing_tool",
									Arguments: `{}`,
								},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
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
	wantErr := errors.New("execute_read: Unknown column 'x'; 请先 describe_table 再重试。")
	_ = reg.Register(tool.Tool{
		Name: "failing_tool",
		Execute: func(context.Context, map[string]any) (any, error) {
			return nil, wantErr
		},
	})

	gen, err := client.ChatWithTools(context.Background(), []Message{{Role: "user", Content: "test"}}, reg)
	if err != nil {
		t.Fatalf("ChatWithTools must not return error when tool fails; err: %v", err)
	}
	if gen == nil {
		t.Fatal("expected non-nil Generation")
	}
	if !strings.HasPrefix(gen.Text, "error: ") {
		t.Fatalf("expected Text to start with 'error: ', got %q", gen.Text)
	}
	if !strings.Contains(gen.Text, wantErr.Error()) {
		t.Fatalf("expected Text to contain tool error, got %q", gen.Text)
	}
	step, _ := gen.Raw.(ToolStep)
	if !step.Used || step.ToolName != "failing_tool" {
		t.Fatalf("expected Used=true, ToolName=failing_tool, got %+v", step)
	}
}
