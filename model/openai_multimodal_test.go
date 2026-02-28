package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestOpenAIClient_Chat_MultimodalMessage(t *testing.T) {
	// 模拟 OpenAI /chat/completions 端点，检查请求中的 MultiContent。
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request error: %v", err)
		}
		if len(body.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(body.Messages))
		}
		msg := body.Messages[0]
		if len(msg.MultiContent) != 2 {
			t.Fatalf("expected 2 content parts, got %d", len(msg.MultiContent))
		}
		if msg.MultiContent[0].Type != openai.ChatMessagePartTypeText || msg.MultiContent[0].Text != "look at this image" {
			t.Fatalf("unexpected first part: %#v", msg.MultiContent[0])
		}
		if msg.MultiContent[1].Type != openai.ChatMessagePartTypeImageURL || msg.MultiContent[1].ImageURL == nil || msg.MultiContent[1].ImageURL.URL != "https://example.com/image.png" {
			t.Fatalf("unexpected second part: %#v", msg.MultiContent[1])
		}

		// 返回一个简单的响应。
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleAssistant,
						Content: "ok",
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
		apiKey:     "test",
		client:     openai.NewClientWithConfig(cfg),
		model:      "gpt-4o-mini",
		httpClient: ts.Client(),
		baseURL:    ts.URL,
	}

	msgs := []Message{
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "", // 内容由 Parts 提供
			Parts: []ContentPart{
				{Type: ContentTypeText, Text: "look at this image"},
				{Type: ContentTypeImageURL, URL: "https://example.com/image.png"},
			},
		},
	}

	gen, err := client.Chat(context.Background(), msgs)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if gen == nil || gen.Text != "ok" {
		t.Fatalf("unexpected generation: %#v", gen)
	}
}
