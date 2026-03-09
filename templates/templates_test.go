package templates

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sath/agent"
	"github.com/sath/config"
	"github.com/sath/memory"
	"github.com/sath/middleware"
	"github.com/sath/model"
	"github.com/sath/skills"
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

// TestBuildSkillsSummary 验证任务 7：Skills 摘要格式与数量限制。
func TestBuildSkillsSummary(t *testing.T) {
	empty := BuildSkillsSummary(nil, 8)
	if empty != "" {
		t.Fatalf("expected empty for nil skills, got %q", empty)
	}
	empty2 := BuildSkillsSummary([]skills.SkillMeta{}, 8)
	if empty2 != "" {
		t.Fatalf("expected empty for zero skills, got %q", empty2)
	}

	all := []skills.SkillMeta{
		{Name: "skill-a", Description: "描述 A"},
		{Name: "skill-b", Description: "描述 B"},
	}
	out := BuildSkillsSummary(all, 8)
	if !strings.Contains(out, "【可用 Skills") || !strings.Contains(out, "load_skill") {
		t.Fatalf("summary should contain header and load_skill hint: %s", out)
	}
	if !strings.Contains(out, "skill-a") || !strings.Contains(out, "描述 A") {
		t.Fatalf("summary should list skill-a: %s", out)
	}
	// maxCount 2
	out2 := BuildSkillsSummary(all, 2)
	if !strings.Contains(out2, "skill-a") && !strings.Contains(out2, "skill-b") {
		t.Fatalf("maxCount 2 should show both: %s", out2)
	}
	// maxCount <= 0 默认 8
	out3 := BuildSkillsSummary(all, 0)
	if out3 == "" {
		t.Fatal("maxCount 0 should default and produce output")
	}
}
