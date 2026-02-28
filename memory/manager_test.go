package memory

import (
	"context"
	"testing"

	"github.com/sath/model"
)

type stubModel struct {
	lastPrompt string
}

func (s *stubModel) Generate(ctx context.Context, prompt string, opts ...model.Option) (*model.Generation, error) {
	_ = ctx
	_ = opts
	s.lastPrompt = prompt
	return &model.Generation{Text: "summary"}, nil
}

func (s *stubModel) Chat(ctx context.Context, messages []model.Message, opts ...model.Option) (*model.Generation, error) {
	return &model.Generation{Text: "chat"}, nil
}

func (s *stubModel) Embed(ctx context.Context, texts []string, opts ...model.Option) ([]model.Embedding, error) {
	out := make([]model.Embedding, len(texts))
	for i, t := range texts {
		out[i] = model.Embedding{Vector: []float32{float32(len(t))}}
	}
	return out, nil
}

func TestManager_AddMessage_TriggersSummaryAndVector(t *testing.T) {
	ctx := context.Background()
	m := &stubModel{}
	vec := NewInMemoryVectorStore()
	sum := NewSummaryMemory(m, 10)

	mgr := NewManager(m, vec, sum, ManagerConfig{MaxShortHistory: 3})

	// 连续写入 3 条消息，期望：
	// - 第 3 条触发一次摘要（短期被清空）
	// - 向量记忆中写入 3 条记录
	for i := 0; i < 3; i++ {
		msg := model.Message{Role: "user", Content: "msg"}
		if err := mgr.AddMessage(ctx, "conv-1", msg, map[string]any{"index": i}); err != nil {
			t.Fatalf("AddMessage error: %v", err)
		}
	}

	// 短期记忆应被清空
	short, err := mgr.ShortRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ShortRecent error: %v", err)
	}
	if len(short) != 0 {
		t.Fatalf("expected short memory cleared after summary, got %d entries", len(short))
	}

	// 摘要应存在
	summaries := mgr.Summaries()
	if len(summaries) == 0 {
		t.Fatalf("expected at least one summary entry")
	}

	// 向量记忆中应有 3 条记录
	res, err := vec.Search(ctx, []float32{3}, 10)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("expected 3 vector entries, got %d", len(res))
	}
}
