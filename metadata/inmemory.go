package metadata

import (
	"context"
	"sync"
)

var _ Store = (*InMemoryStore)(nil)

// InMemoryStore 内存实现的 Store，按 datasourceID 缓存 Schema。
type InMemoryStore struct {
	mu   sync.RWMutex
	data map[string]*Schema
}

// NewInMemoryStore 返回新的空 InMemoryStore。
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{data: make(map[string]*Schema)}
}

// GetSchema 实现 Store。
func (s *InMemoryStore) GetSchema(ctx context.Context, datasourceID string) (*Schema, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sch, ok := s.data[datasourceID]
	if !ok || sch == nil {
		return nil, nil
	}
	return sch, nil
}

// Refresh 实现 Store：调用 fetch 并将结果写入缓存。
func (s *InMemoryStore) Refresh(ctx context.Context, datasourceID string, fetch FetchFunc) error {
	sch, err := fetch()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]*Schema)
	}
	s.data[datasourceID] = sch
	return nil
}

// GetTable 实现 Store：从已缓存的 Schema 中按表名查找。
func (s *InMemoryStore) GetTable(ctx context.Context, datasourceID, tableName string) (*Table, error) {
	sch, err := s.GetSchema(ctx, datasourceID)
	if err != nil || sch == nil {
		return nil, err
	}
	for _, t := range sch.Tables {
		if t != nil && t.Name == tableName {
			return t, nil
		}
	}
	return nil, nil
}
