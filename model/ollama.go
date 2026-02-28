package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

// OllamaClient 提供对本地 Ollama /api/generate 接口的最小封装。
// 适用于简单文本对话与生成场景。
type OllamaClient struct {
	httpClient *http.Client
	baseURL    string
	model      string
}

type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaGenerateResponse struct {
	Model      string `json:"model"`
	Response   string `json:"response"`
	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason,omitempty"`
}

// NewOllamaClient 从环境变量构建一个默认客户端。
// - OLLAMA_MODEL：默认模型名称（必填）；
// - OLLAMA_BASE_URL：服务地址，默认为 http://localhost:11434。
func NewOllamaClient() (*OllamaClient, error) {
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		return nil, errors.New("missing OLLAMA_MODEL")
	}
	base := os.Getenv("OLLAMA_BASE_URL")
	if base == "" {
		base = "http://localhost:11434"
	}
	return &OllamaClient{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    base,
		model:      modelName,
	}, nil
}

// Generate 使用 Ollama /api/generate 执行一次非流式生成。
func (c *OllamaClient) Generate(ctx context.Context, prompt string, opts ...Option) (*Generation, error) {
	if prompt == "" {
		return nil, errors.New("prompt is empty")
	}
	reqBody := ollamaGenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama status %d", resp.StatusCode)
	}

	var decoded ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &Generation{
		Text: decoded.Response,
		Raw:  decoded,
	}, nil
}

// Chat 的简单实现：将多轮消息拼成一个 prompt，调用 Generate。
func (c *OllamaClient) Chat(ctx context.Context, messages []Message, opts ...Option) (*Generation, error) {
	if len(messages) == 0 {
		return nil, errors.New("messages is empty")
	}
	var b bytes.Buffer
	for _, m := range messages {
		if m.Role != "" {
			b.WriteString(m.Role)
			b.WriteString(": ")
		}
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return c.Generate(ctx, b.String(), opts...)
}

// Embed 暂不支持，本地模型通常需要单独的 embedding 模型或配置。
func (c *OllamaClient) Embed(ctx context.Context, texts []string, opts ...Option) ([]Embedding, error) {
	_ = ctx
	_ = texts
	_ = opts
	return nil, errors.New("ollama: Embed not implemented")
}
