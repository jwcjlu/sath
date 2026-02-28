package memory

import (
	"context"
	"testing"
)

func TestInMemoryVectorStore_AddAndSearch(t *testing.T) {
	store := NewInMemoryVectorStore()
	ctx := context.Background()

	v1 := VectorEntry{ID: "a", Vector: []float32{1, 0}}
	v2 := VectorEntry{ID: "b", Vector: []float32{0, 1}}
	v3 := VectorEntry{ID: "c", Vector: []float32{0.7, 0.7}}

	_ = store.Add(ctx, v1)
	_ = store.Add(ctx, v2)
	_ = store.Add(ctx, v3)

	// 查询向量接近 (1,0)，期望 a 最相关，其次 c。
	res, err := store.Search(ctx, []float32{1, 0}, 2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	if res[0].ID != "a" {
		t.Fatalf("expected first result to be 'a', got %s", res[0].ID)
	}
}
