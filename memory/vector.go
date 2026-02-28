package memory

import (
	"context"
	"math"

	"github.com/sath/model"
)

// VectorEntry 表示一条向量记忆。
type VectorEntry struct {
	ID       string
	Vector   []float32
	Metadata map[string]any
}

// VectorStore 定义向量记忆接口，不关心向量如何生成。
type VectorStore interface {
	Add(ctx context.Context, entry VectorEntry) error
	Search(ctx context.Context, query []float32, k int) ([]VectorEntry, error)
	Clear(ctx context.Context) error
}

// InMemoryVectorStore 使用内存切片实现的简单向量记忆，适合作为默认实现或测试用途。
type InMemoryVectorStore struct {
	entries []VectorEntry
}

func NewInMemoryVectorStore() *InMemoryVectorStore {
	return &InMemoryVectorStore{
		entries: make([]VectorEntry, 0),
	}
}

func (s *InMemoryVectorStore) Add(_ context.Context, entry VectorEntry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *InMemoryVectorStore) Search(_ context.Context, query []float32, k int) ([]VectorEntry, error) {
	if k <= 0 {
		return nil, nil
	}
	type scored struct {
		entry VectorEntry
		score float32
	}
	var scoredList []scored
	for _, e := range s.entries {
		if len(e.Vector) == 0 || len(query) == 0 {
			continue
		}
		sim := cosineSimilarity(e.Vector, query)
		scoredList = append(scoredList, scored{entry: e, score: sim})
	}
	if len(scoredList) == 0 {
		return nil, nil
	}
	// 简单选择前 k 个相似度最高的结果（O(nk) 选择，避免引入排序依赖）。
	result := make([]VectorEntry, 0, min(k, len(scoredList)))
	for i := 0; i < k && len(scoredList) > 0; i++ {
		// 选出当前最高分
		bestIdx := 0
		for j := 1; j < len(scoredList); j++ {
			if scoredList[j].score > scoredList[bestIdx].score {
				bestIdx = j
			}
		}
		result = append(result, scoredList[bestIdx].entry)
		// 移除已选项
		scoredList = append(scoredList[:bestIdx], scoredList[bestIdx+1:]...)
	}
	return result, nil
}

func (s *InMemoryVectorStore) Clear(_ context.Context) error {
	s.entries = s.entries[:0]
	return nil
}

func cosineSimilarity(a, b []float32) float32 {
	ln := len(a)
	if len(b) < ln {
		ln = len(b)
	}
	if ln == 0 {
		return 0
	}
	var dot, na, nb float32
	for i := 0; i < ln; i++ {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(na*nb)))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// EmbedAndAdd 是一个辅助方法，方便在使用 model.Model 时向向量记忆中写入文本。
// 它不属于 VectorStore 接口本身，调用方可按需使用。
func EmbedAndAdd(ctx context.Context, m model.Model, store VectorStore, id string, text string, meta map[string]any) error {
	embs, err := m.Embed(ctx, []string{text})
	if err != nil {
		return err
	}
	if len(embs) == 0 {
		return nil
	}
	return store.Add(ctx, VectorEntry{
		ID:       id,
		Vector:   embs[0].Vector,
		Metadata: meta,
	})
}
