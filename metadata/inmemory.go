package metadata

import (
	"context"
	"sync"
)

// FetchFunc 抽象真实元数据拉取逻辑，便于复用与测试
type FetchFunc func(ctx context.Context) (*Schema, error)

// InMemoryStore 简单的内存实现，持有一次拉取结果
type InMemoryStore struct {
	mu     sync.RWMutex
	schema *Schema
	fetch  FetchFunc
}

// NewInMemoryStore 创建基于给定 FetchFunc 的内存 Store
func NewInMemoryStore(fetch FetchFunc) *InMemoryStore {
	return &InMemoryStore{
		fetch: fetch,
	}
}

func (s *InMemoryStore) GetSchema(_ context.Context) (*Schema, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.schema == nil {
		return nil, nil
	}
	cp := *s.schema
	return &cp, nil
}

func (s *InMemoryStore) Refresh(ctx context.Context) (*Schema, error) {
	if s.fetch == nil {
		return nil, nil
	}

	schema, err := s.fetch(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.schema = schema
	s.mu.Unlock()

	cp := *schema
	return &cp, nil
}

func (s *InMemoryStore) GetTable(ctx context.Context, name string) (*Table, error) {
	schema, err := s.GetSchema(ctx)
	if err != nil || schema == nil {
		return nil, err
	}
	for _, t := range schema.Tables {
		if t.Name == name {
			cp := t
			return &cp, nil
		}
	}
	return nil, nil
}
