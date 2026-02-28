package memory

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkInMemoryVectorStore_Search(b *testing.B) {
	ctx := context.Background()
	store := NewInMemoryVectorStore()
	// 插入 100 条，向量维度 64
	dim := 64
	for i := 0; i < 100; i++ {
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = float32(i + j)
		}
		_ = store.Add(ctx, VectorEntry{ID: fmt.Sprintf("id-%d", i), Vector: vec})
	}
	query := make([]float32, dim)
	for i := range query {
		query[i] = 1
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Search(ctx, query, 10)
	}
}
