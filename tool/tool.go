package tool

import (
	"context"
	"errors"
	"sync"
)

// RegisterToolOptions 用于注册工具时的可选覆盖（如按数据源类型的描述文案）。
// 若 Description 非空，则覆盖该工具的默认 Description（模型可见）。
type RegisterToolOptions struct {
	Description string
}

// ExecuteFunc 是工具的执行函数签名。
type ExecuteFunc func(ctx context.Context, params map[string]any) (any, error)

// Tool 描述一个可被 Agent 调用的工具。
type Tool struct {
	Name        string
	Description string
	// Parameters 描述参数结构，遵循 JSON Schema 风格（V0.1 可简化为任意 map）。
	Parameters map[string]any
	Execute    ExecuteFunc
}

// Registry 维护一组可用工具。
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry 创建一个空的工具注册表。
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 向注册表中添加一个工具。
func (r *Registry) Register(t Tool) error {
	if t.Name == "" {
		return errors.New("tool name is empty")
	}
	if t.Execute == nil {
		return errors.New("tool execute is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name]; exists {
		return errors.New("tool already registered: " + t.Name)
	}
	r.tools[t.Name] = t
	return nil
}

// Get 根据名称获取工具。
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List 返回当前注册的所有工具。
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}
