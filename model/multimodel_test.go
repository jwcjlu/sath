package model

import (
	"context"
	"testing"
)

type stubModel struct {
	name           string
	lastPrompt     string
	lastMessages   []Message
	generateCalled int
	chatCalled     int
	embedCalled    int
}

func (s *stubModel) Generate(ctx context.Context, prompt string, opts ...Option) (*Generation, error) {
	_ = ctx
	s.generateCalled++
	s.lastPrompt = prompt
	return &Generation{Text: s.name + ":generate"}, nil
}

func (s *stubModel) Chat(ctx context.Context, messages []Message, opts ...Option) (*Generation, error) {
	_ = ctx
	s.chatCalled++
	s.lastMessages = append([]Message(nil), messages...)
	return &Generation{Text: s.name + ":chat"}, nil
}

func (s *stubModel) Embed(ctx context.Context, texts []string, opts ...Option) ([]Embedding, error) {
	_ = ctx
	s.embedCalled++
	out := make([]Embedding, len(texts))
	for i := range texts {
		out[i] = Embedding{Vector: []float32{float32(len(texts[i]))}}
	}
	return out, nil
}

func TestMultiModel_SelectByName_Chat(t *testing.T) {
	a := &stubModel{name: "a"}
	b := &stubModel{name: "b"}

	mm, err := NewMultiModelFromMap("a", map[string]Model{
		"a": a,
		"b": b,
	}, StrategyByName)
	if err != nil {
		t.Fatalf("NewMultiModelFromMap error: %v", err)
	}

	// 默认使用 a
	_, err = mm.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}}, WithTemperature(0.1))
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if a.chatCalled != 1 || b.chatCalled != 0 {
		t.Fatalf("expected a used once, b unused; got a=%d b=%d", a.chatCalled, b.chatCalled)
	}

	// 显式选择 b
	_, err = mm.Chat(context.Background(), []Message{{Role: "user", Content: "hi2"}}, WithModelName("b"))
	if err != nil {
		t.Fatalf("Chat with model b error: %v", err)
	}
	if a.chatCalled != 1 || b.chatCalled != 1 {
		t.Fatalf("expected a=1, b=1; got a=%d b=%d", a.chatCalled, b.chatCalled)
	}
}

func TestMultiModel_SelectByName_Invalid(t *testing.T) {
	a := &stubModel{name: "a"}
	mm, err := NewMultiModelFromMap("a", map[string]Model{"a": a}, StrategyByName)
	if err != nil {
		t.Fatalf("NewMultiModelFromMap error: %v", err)
	}

	_, err = mm.Chat(context.Background(), []Message{{Role: "user", Content: "x"}}, WithModelName("not-exist"))
	if err == nil {
		t.Fatalf("expected error for unknown model name")
	}
}
