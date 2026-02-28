package templates

import (
	"context"
	"strings"

	"github.com/sath/agent"
	"github.com/sath/memory"
	"github.com/sath/middleware"
	"github.com/sath/model"
)

// RAGConfig 控制文档问答 Agent 的行为。
type RAGConfig struct {
	TopK int
}

// NewRAGHandler 构建一个简单的文档问答（RAG）处理器。
// 要求调用方事先将文档内容按片段写入 VectorStore 的 Metadata["content"] 中。
func NewRAGHandler(m model.Model, store memory.VectorStore, cfg RAGConfig, mws ...middleware.Middleware) middleware.Handler {
	if cfg.TopK <= 0 {
		cfg.TopK = 3
	}

	final := func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		if req == nil || len(req.Messages) == 0 {
			return &agent.Response{Text: ""}, nil
		}
		// 使用最后一条用户消息作为问题。
		var question string
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				question = req.Messages[i].Content
				break
			}
		}
		if question == "" {
			question = req.Messages[len(req.Messages)-1].Content
		}

		// 基于向量记忆检索相关文档。
		var contextText string
		if store != nil {
			embs, err := m.Embed(ctx, []string{question})
			if err == nil && len(embs) > 0 {
				vecs, err := store.Search(ctx, embs[0].Vector, cfg.TopK)
				if err == nil {
					var parts []string
					for _, v := range vecs {
						if s, ok := v.Metadata["content"].(string); ok && s != "" {
							parts = append(parts, s)
						}
					}
					if len(parts) > 0 {
						contextText = strings.Join(parts, "\n\n---\n\n")
					}
				}
			}
		}

		// 构造带检索上下文的对话，交给底层模型回答。
		var messages []model.Message
		if contextText != "" {
			messages = append(messages, model.Message{
				Role: "system",
				Content: "你是一个文档问答助手。根据以下提供的文档内容回答用户问题，" +
					"优先基于文档内容作答，如无法从文档中找到答案，请明确说明。",
			})
			messages = append(messages, model.Message{
				Role:    "system",
				Content: "文档内容如下：\n" + contextText,
			})
		}
		messages = append(messages, model.Message{Role: "user", Content: question})

		gen, err := m.Chat(ctx, messages)
		if err != nil {
			return nil, err
		}
		return &agent.Response{Text: gen.Text}, nil
	}

	return middleware.Chain(final, mws...)
}
