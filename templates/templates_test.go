package templates

import (
	"context"
	"testing"
	"time"

	"github.com/sath/agent"
	"github.com/sath/config"
	"github.com/sath/memory"
	"github.com/sath/middleware"
	"github.com/sath/model"
)

type stubModel struct {
	lastMessages []model.Message
	reply        string
}

func (s *stubModel) Generate(ctx context.Context, prompt string, opts ...model.Option) (*model.Generation, error) {
	_ = ctx
	_ = opts
	return &model.Generation{Text: s.reply}, nil
}

func (s *stubModel) Chat(ctx context.Context, messages []model.Message, opts ...model.Option) (*model.Generation, error) {
	_ = ctx
	_ = opts
	s.lastMessages = append([]model.Message(nil), messages...)
	return &model.Generation{Text: s.reply}, nil
}

func (s *stubModel) Embed(ctx context.Context, texts []string, opts ...model.Option) ([]model.Embedding, error) {
	_ = ctx
	_ = opts
	out := make([]model.Embedding, len(texts))
	for i := range texts {
		out[i] = model.Embedding{Vector: []float32{float32(len(texts[i]))}}
	}
	return out, nil
}

func TestNewChatAgentHandler(t *testing.T) {
	m := &stubModel{reply: "hello"}
	mem := memory.NewBufferMemory(5)
	h := NewChatAgentHandler(m, mem)

	req := &agent.Request{
		Messages: []model.Message{{Role: "user", Content: "hi"}},
	}
	resp, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp == nil || resp.Text != "hello" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

type stubVectorStore struct {
	entries []memory.VectorEntry
}

func (s *stubVectorStore) Add(ctx context.Context, e memory.VectorEntry) error {
	s.entries = append(s.entries, e)
	return nil
}

func (s *stubVectorStore) Search(ctx context.Context, q []float32, k int) ([]memory.VectorEntry, error) {
	if len(s.entries) == 0 {
		return nil, nil
	}
	if k <= 0 || k >= len(s.entries) {
		return s.entries, nil
	}
	return s.entries[:k], nil
}

func (s *stubVectorStore) Clear(ctx context.Context) error {
	s.entries = nil
	return nil
}

func TestNewRAGHandler(t *testing.T) {
	m := &stubModel{reply: "answer based on docs"}
	store := &stubVectorStore{
		entries: []memory.VectorEntry{
			{
				ID:     "doc1",
				Vector: []float32{1},
				Metadata: map[string]any{
					"content": "这是一段文档内容。",
				},
			},
		},
	}

	h := NewRAGHandler(m, store, RAGConfig{TopK: 1}, middleware.MetricsMiddleware)

	req := &agent.Request{
		Messages: []model.Message{
			{Role: "user", Content: "根据文档回答一个问题"},
		},
		Metadata: map[string]any{"agent_name": "rag"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := h(ctx, req)
	if err != nil {
		t.Fatalf("RAG handler error: %v", err)
	}
	if resp == nil || resp.Text != "answer based on docs" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestNewChatAgentHandlerFromConfig_InvalidModel(t *testing.T) {
	cfg := config.Config{ModelName: "nonexistent/fake", MaxHistory: 5}
	_, err := NewChatAgentHandlerFromConfig(cfg, DefaultMiddlewareMap())
	if err == nil {
		t.Fatal("expected error for invalid model name")
	}
}
