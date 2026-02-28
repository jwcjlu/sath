package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/sath/memory"
	"github.com/sath/model"
)

type fakeModel struct {
	lastMessages []model.Message
	replyText    string
	err          error
}

func (f *fakeModel) Generate(ctx context.Context, prompt string, opts ...model.Option) (*model.Generation, error) {
	_ = ctx
	if f.err != nil {
		return nil, f.err
	}
	f.lastMessages = append(f.lastMessages, model.Message{Role: "user", Content: prompt})
	return &model.Generation{Text: f.replyText}, nil
}

func (f *fakeModel) Chat(ctx context.Context, msgs []model.Message, opts ...model.Option) (*model.Generation, error) {
	_ = ctx
	f.lastMessages = append([]model.Message(nil), msgs...)
	if f.err != nil {
		return nil, f.err
	}
	return &model.Generation{Text: f.replyText}, nil
}

func (f *fakeModel) Embed(ctx context.Context, texts []string, opts ...model.Option) ([]model.Embedding, error) {
	_ = ctx
	_ = opts
	out := make([]model.Embedding, len(texts))
	for i := range texts {
		out[i] = model.Embedding{Vector: []float32{1, 2, 3}}
	}
	return out, nil
}

func TestChatAgent_Run_NilRequest(t *testing.T) {
	m := &fakeModel{replyText: "hi"}
	mem := memory.NewBufferMemory(5)
	a := NewChatAgent(m, mem)

	resp, err := a.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response for nil request, got %#v", resp)
	}
}

func TestChatAgent_Run_UsesHistoryAndStoresReply(t *testing.T) {
	ctx := context.Background()
	m := &fakeModel{replyText: "assistant reply"}
	mem := memory.NewBufferMemory(5)

	// 预先写入一条历史消息
	_ = mem.Add(ctx, memory.Entry{
		Message: model.Message{Role: "user", Content: "old"},
	})

	a := NewChatAgent(m, mem, WithMaxHistory(5))

	req := &Request{
		Messages: []model.Message{
			{Role: "user", Content: "new"},
		},
	}
	resp, err := a.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if resp == nil || resp.Text != "assistant reply" {
		t.Fatalf("unexpected response: %#v", resp)
	}

	// 验证模型收到的消息包含历史 + 当前
	if len(m.lastMessages) != 2 {
		t.Fatalf("expected 2 messages sent to model, got %d", len(m.lastMessages))
	}
	if m.lastMessages[0].Content != "old" || m.lastMessages[1].Content != "new" {
		t.Fatalf("unexpected messages: %#v", m.lastMessages)
	}

	// 验证回复写入记忆
	entries, err := mem.GetRecent(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecent error: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Message.Role == "assistant" && e.Message.Content == "assistant reply" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("assistant reply not stored in memory")
	}
}

func TestChatAgent_Run_ModelError(t *testing.T) {
	expectErr := errors.New("model failed")
	m := &fakeModel{err: expectErr}
	mem := memory.NewBufferMemory(5)
	a := NewChatAgent(m, mem)

	req := &Request{
		Messages: []model.Message{
			{Role: "user", Content: "hello"},
		},
	}
	resp, err := a.Run(context.Background(), req)
	if err == nil || !errors.Is(err, expectErr) {
		t.Fatalf("expected error %v, got %v", expectErr, err)
	}
	if resp != nil {
		t.Fatalf("expected nil response on error, got %#v", resp)
	}
}
