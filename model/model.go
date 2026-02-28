package model

import (
	"context"
)

// ContentType 表示多模态消息中单个内容块的类型。
type ContentType string

const (
	ContentTypeText     ContentType = "text"
	ContentTypeImageURL ContentType = "image_url"
	ContentTypeAudioURL ContentType = "audio_url"
	ContentTypeVideoURL ContentType = "video_url"
)

// ContentPart 表示多模态消息中的一个内容块。
// 为保持与现有实现兼容，纯文本场景仍主要使用 Message.Content 字段，
// 多模态场景可通过 Parts 扩展。
type ContentPart struct {
	Type ContentType
	// Text 文本内容，当 Type=text 时使用。
	Text string
	// URL 资源地址，当 Type 为 *_url 时使用。
	URL string
	// Metadata 预留扩展位，例如文件名、MIME 类型等。
	Metadata map[string]any
}

// Message 表示单条对话消息，多模态支持通过 Parts 实现。
type Message struct {
	Role string
	// Content 为纯文本场景保留，等价于单一 text 内容。
	Content string
	// Parts 为多模态内容列表，允许同时包含文本、图片等。
	Parts []ContentPart
	// Metadata 预留扩展位，例如消息 ID、时间戳等。
	Metadata map[string]any
}

// TokenUsage 表示一次调用的 token 消耗（可选，由适配器在支持时填充）。
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// Generation 表示一次模型生成结果（文本）。
type Generation struct {
	Text string
	// Raw 用于保存底层模型的完整响应，便于调试或扩展。
	Raw any
	// TokenUsage 当底层 API 返回用量时由适配器填充，用于指标上报。
	TokenUsage *TokenUsage
}

// Embedding 向量表示。
type Embedding struct {
	Vector []float32
	// Raw 用于保存底层向量响应，便于调试或扩展。
	Raw any
}

// ToolStep 表示一次基于 tools API 的决策结果。
// Used 为 false 时表示本轮未调用任何工具（通常意味着直接给出答案）。
type ToolStep struct {
	Used        bool
	ToolName    string
	Observation any
}

// Model 定义统一的模型接口。
// V0.2 起提供 Generate / Chat / Embed 三种能力，具体实现可按需支持。
type Model interface {
	// Generate 适用于单轮文本生成场景。
	Generate(ctx context.Context, prompt string, opts ...Option) (*Generation, error)
	// Chat 适用于多轮对话场景。
	Chat(ctx context.Context, messages []Message, opts ...Option) (*Generation, error)
	// Embed 将一组文本转换为向量表示。
	Embed(ctx context.Context, texts []string, opts ...Option) ([]Embedding, error)
}

// Option 为模型调用提供可选参数（如温度、最大 Token 等）。
type Option func(*CallConfig)

// CallConfig 保存一次模型调用的可选配置。
type CallConfig struct {
	Temperature float32
	MaxTokens   int
	ModelName   string
}

// ApplyOptions 根据可选参数构建最终配置。
func ApplyOptions(opts ...Option) *CallConfig {
	cfg := &CallConfig{
		Temperature: 0.7,
		MaxTokens:   1024,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func WithTemperature(t float32) Option {
	return func(c *CallConfig) {
		c.Temperature = t
	}
}

func WithMaxTokens(n int) Option {
	return func(c *CallConfig) {
		if n > 0 {
			c.MaxTokens = n
		}
	}
}

func WithModelName(name string) Option {
	return func(c *CallConfig) {
		if name != "" {
			c.ModelName = name
		}
	}
}
