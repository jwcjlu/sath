package memory

import (
	"context"
	"testing"

	"github.com/sath/model"
)

func TestBufferMemory_AddAndGetRecent(t *testing.T) {
	mem := NewBufferMemory(2)
	ctx := context.Background()

	_ = mem.Add(ctx, Entry{Message: model.Message{Role: "user", Content: "1"}})
	_ = mem.Add(ctx, Entry{Message: model.Message{Role: "user", Content: "2"}})
	_ = mem.Add(ctx, Entry{Message: model.Message{Role: "user", Content: "3"}})

	entries, err := mem.GetRecent(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecent error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Message.Content != "2" || entries[1].Message.Content != "3" {
		t.Fatalf("unexpected entries order: %#v", entries)
	}
}
