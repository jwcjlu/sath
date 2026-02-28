package model

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const defaultOpenAIModel = "gpt-3.5-turbo"

type OpenAIClient struct {
	apiKey     string
	client     *openai.Client
	model      string
	httpClient *http.Client
	baseURL    string
}

// NewOpenAIClient 从环境变量构建一个默认客户端。
// 依赖 OPENAI_API_KEY，可选 OPENAI_BASE_URL、OPENAI_MODEL。
func NewOpenAIClient() (*OpenAIClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("missing OPENAI_API_KEY")
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = defaultOpenAIModel
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}
	cfg.HTTPClient = httpClient

	return &OpenAIClient{
		apiKey:     apiKey,
		client:     openai.NewClientWithConfig(cfg),
		model:      model,
		httpClient: httpClient,
		baseURL:    baseURL,
	}, nil
}

// Generate 使用单轮对话形式实现简单文本生成。
func (c *OpenAIClient) Generate(ctx context.Context, prompt string, opts ...Option) (*Generation, error) {
	msgs := []Message{
		{Role: openai.ChatMessageRoleUser, Content: prompt},
	}
	return c.Chat(ctx, msgs, opts...)
}

func (c *OpenAIClient) Chat(ctx context.Context, messages []Message, opts ...Option) (*Generation, error) {
	if len(messages) == 0 {
		return nil, errors.New("messages is empty")
	}

	callCfg := ApplyOptions(opts...)
	modelName := c.model
	if callCfg.ModelName != "" {
		modelName = callCfg.ModelName
	}

	req := openai.ChatCompletionRequest{
		Model:       modelName,
		Messages:    make([]openai.ChatCompletionMessage, 0, len(messages)),
		Temperature: float32(callCfg.Temperature),
		MaxTokens:   callCfg.MaxTokens,
	}
	for _, m := range messages {
		msg := openai.ChatCompletionMessage{
			Role: m.Role,
		}
		// 多模态支持：如果提供了 Parts，则映射为 MultiContent；否则仅使用 Content。
		if len(m.Parts) > 0 {
			mc := make([]openai.ChatMessagePart, 0, len(m.Parts))
			for _, p := range m.Parts {
				switch p.Type {
				case ContentTypeText:
					mc = append(mc, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeText,
						Text: p.Text,
					})
				case ContentTypeImageURL:
					mc = append(mc, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: p.URL,
						},
					})
				default:
					// 其他类型暂不处理，留给后续扩展。
				}
			}
			if len(mc) > 0 {
				msg.MultiContent = mc
			}
		}
		// 保持向后兼容：仍然填充 Content 字段，用于文本-only 场景或没有 Parts 时。
		if msg.MultiContent == nil {
			msg.Content = m.Content
		}
		req.Messages = append(req.Messages, msg)
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	text := resp.Choices[0].Message.Content
	gen := &Generation{Text: text, Raw: resp}
	if resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
		gen.TokenUsage = &TokenUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	return gen, nil
}

// Embed 使用 OpenAI Embeddings API 生成文本向量。
func (c *OpenAIClient) Embed(ctx context.Context, texts []string, opts ...Option) ([]Embedding, error) {
	if len(texts) == 0 {
		return nil, errors.New("texts is empty")
	}

	// 目前暂不通过 Option 配置 embedding 模型，直接使用默认模型。
	req := openai.EmbeddingRequest{
		Input: texts,
		Model: openai.SmallEmbedding3,
	}

	resp, err := c.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, errors.New("no embeddings in response")
	}

	out := make([]Embedding, 0, len(resp.Data))
	for _, e := range resp.Data {
		cp := make([]float32, len(e.Embedding))
		copy(cp, e.Embedding)
		out = append(out, Embedding{
			Vector: cp,
			Raw:    e,
		})
	}
	return out, nil
}
