package intent

import (
	"encoding/json"
	"sync"
)

// DataSessionContext 数据对话会话上下文：当前数据源、上轮 DSL、表名等，供多轮与改写使用。
type DataSessionContext struct {
	DatasourceID string `json:"datasource_id,omitempty"`
	DefaultSchema string `json:"default_schema,omitempty"`
	LastDSL      string `json:"last_dsl,omitempty"`
	LastTable    string `json:"last_table,omitempty"`
	LastIntent   Intent `json:"last_intent,omitempty"`
	// PendingConfirm 待用户确认的 DSL；确认后清空。
	PendingConfirmDSL string `json:"pending_confirm_dsl,omitempty"`
	PendingConfirmDesc string `json:"pending_confirm_desc,omitempty"`
	PendingConfirmAt   int64  `json:"pending_confirm_at,omitempty"` // Unix 秒，用于超时
}

// DataSessionStore 按 sessionID 存取数据对话上下文。
type DataSessionStore interface {
	Get(sessionID string) (*DataSessionContext, error)
	Set(sessionID string, ctx *DataSessionContext) error
}

// InMemoryDataSessionStore 内存实现，适合单机或测试。
type InMemoryDataSessionStore struct {
	mu   sync.RWMutex
	data map[string]*DataSessionContext
}

// NewInMemoryDataSessionStore 返回新的空 Store。
func NewInMemoryDataSessionStore() *InMemoryDataSessionStore {
	return &InMemoryDataSessionStore{data: make(map[string]*DataSessionContext)}
}

// Get 实现 DataSessionStore。
func (s *InMemoryDataSessionStore) Get(sessionID string) (*DataSessionContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.data[sessionID]
	if !ok || c == nil {
		return nil, nil
	}
	return c, nil
}

// Set 实现 DataSessionStore。
func (s *InMemoryDataSessionStore) Set(sessionID string, ctx *DataSessionContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]*DataSessionContext)
	}
	s.data[sessionID] = ctx
	return nil
}

// GetDataContextFromMetadata 从 Request.Metadata 中读取 data_context（JSON）。
func GetDataContextFromMetadata(metadata map[string]any) (*DataSessionContext, error) {
	if metadata == nil {
		return nil, nil
	}
	v, ok := metadata["data_context"]
	if !ok || v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case *DataSessionContext:
		return t, nil
	case map[string]any:
		var c DataSessionContext
		data, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, err
		}
		return &c, nil
	default:
		return nil, nil
	}
}

// SetDataContextInMetadata 将 data_context 写入 metadata；若 metadata 为 nil 不写入。
func SetDataContextInMetadata(metadata map[string]any, ctx *DataSessionContext) {
	if metadata == nil {
		return
	}
	metadata["data_context"] = ctx
}
