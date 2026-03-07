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
	Parameters any
	Execute    ExecuteFunc
}

// Registry 维护一组可用工具。
type Registry struct {
	mu           sync.RWMutex
	tools        map[string]Tool
	mcpServerIDs map[string]struct{} // 已注册的 MCP 服务 ID，用于按 Skill 使用时幂等注册
}

// NewRegistry 创建一个空的工具注册表。
func NewRegistry() *Registry {
	return &Registry{
		tools:        make(map[string]Tool),
		mcpServerIDs: make(map[string]struct{}),
	}
}

// HasMcpServer 返回该 MCP 服务 ID 是否已在本 Registry 中注册过（用于按 Skill 动态注册时去重）。
func (r *Registry) HasMcpServer(id string) bool {
	if id == "" {
		return false
	}
	r.mu.RLock()
	_, ok := r.mcpServerIDs[id]
	r.mu.RUnlock()
	return ok
}

// MarkMcpServer 标记某 MCP 服务 ID 已在本 Registry 注册，调用方应在成功注册 MCP 工具后调用。
func (r *Registry) MarkMcpServer(id string) {
	if id == "" {
		return
	}
	r.mu.Lock()
	if r.mcpServerIDs == nil {
		r.mcpServerIDs = make(map[string]struct{})
	}
	r.mcpServerIDs[id] = struct{}{}
	r.mu.Unlock()
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
