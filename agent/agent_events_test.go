package agent

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/sath/events"
	"github.com/sath/memory"
	"github.com/sath/model"
)

func TestChatAgent_Run_EmitsLifecycleEvents(t *testing.T) {
	bus := events.NewBus()
	var seen []events.Kind
	var mu sync.Mutex
	bus.Subscribe(false, func(ctx context.Context, e events.Event) {
		mu.Lock()
		seen = append(seen, e.Kind)
		mu.Unlock()
	})

	m := &fakeModel{replyText: "ok"}
	mem := memory.NewBufferMemory(5)
	a := NewChatAgent(m, mem, WithEventBus(bus))

	req := &Request{
		Messages:  []model.Message{{Role: "user", Content: "hi"}},
		RequestID: "req-1",
	}
	_, err := a.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	kinds := append([]events.Kind(nil), seen...)
	mu.Unlock()

	want := []events.Kind{events.RunStarted, events.ModelResponded, events.RunCompleted}
	if len(kinds) != len(want) {
		t.Fatalf("got %d events %v, want %v", len(kinds), kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("event[%d]: got %s, want %s", i, kinds[i], want[i])
		}
	}
}

func TestChatAgent_Run_EmitsRunErrorOnModelFailure(t *testing.T) {
	bus := events.NewBus()
	var errKind events.Kind
	var mu sync.Mutex
	bus.Subscribe(false, func(ctx context.Context, e events.Event) {
		if e.Kind == events.RunError {
			mu.Lock()
			errKind = e.Kind
			mu.Unlock()
		}
	})

	m := &fakeModel{err: errors.New("model failed")}
	mem := memory.NewBufferMemory(5)
	a := NewChatAgent(m, mem, WithEventBus(bus))

	req := &Request{Messages: []model.Message{{Role: "user", Content: "hi"}}}
	_, _ = a.Run(context.Background(), req)

	var got events.Kind
	mu.Lock()
	got = errKind
	mu.Unlock()
	if got != events.RunError {
		t.Fatalf("expected RunError event, got %s", got)
	}
}
