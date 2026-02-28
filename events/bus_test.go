package events

import (
	"context"
	"sync"
	"testing"
)

func TestBus_PublishSyncAndAsync(t *testing.T) {
	bus := NewBus()
	var syncSeen bool
	asyncDone := make(chan struct{})

	bus.Subscribe(false, func(ctx context.Context, e Event) {
		syncSeen = true
	})
	var once sync.Once
	bus.Subscribe(true, func(ctx context.Context, e Event) {
		once.Do(func() { close(asyncDone) })
	})

	ctx := context.Background()
	bus.Publish(ctx, Event{Kind: RunStarted, RequestID: "r1"})

	if !syncSeen {
		t.Error("sync listener did not run")
	}
	<-asyncDone
}
