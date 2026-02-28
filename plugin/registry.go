package plugin

import (
	"sync"

	"github.com/sath/events"
	"github.com/sath/middleware"
	"github.com/sath/model"
	"github.com/sath/tool"
)

// Registry 是插件注册中心，支持模型 Provider、工具、中间件与事件监听器。
// 插件在 init() 中调用本包的 Register* 函数完成注册。
var Registry = new(registry)

type registry struct {
	mu          sync.RWMutex
	tools       []tool.Tool
	middlewares map[string]middleware.Middleware
	listeners   []listenerEntry
}

type listenerEntry struct {
	async bool
	l     events.Listener
}

// RegisterModelProvider 向 model 包注册一个模型 Provider，NewFromIdentifier 会优先使用。
func RegisterModelProvider(provider string, f model.ModelProvider) {
	model.RegisterProvider(provider, f)
}

// RegisterTool 注册一个工具，应用可通过 RegisteredTools() 获取并合并到 tool.Registry。
func RegisterTool(t tool.Tool) {
	Registry.mu.Lock()
	defer Registry.mu.Unlock()
	Registry.tools = append(Registry.tools, t)
}

// RegisteredTools 返回当前已注册的所有插件工具（副本）。
func RegisteredTools() []tool.Tool {
	Registry.mu.RLock()
	defer Registry.mu.RUnlock()
	out := make([]tool.Tool, len(Registry.tools))
	copy(out, Registry.tools)
	return out
}

// RegisterMiddleware 注册一个命名中间件，应用可通过 RegisteredMiddlewares() 按名获取并组装链。
func RegisterMiddleware(name string, mw middleware.Middleware) {
	Registry.mu.Lock()
	defer Registry.mu.Unlock()
	if Registry.middlewares == nil {
		Registry.middlewares = make(map[string]middleware.Middleware)
	}
	Registry.middlewares[name] = mw
}

// RegisteredMiddlewares 返回已注册的中间件映射（副本）。键为名称，值为中间件。
func RegisteredMiddlewares() map[string]middleware.Middleware {
	Registry.mu.RLock()
	defer Registry.mu.RUnlock()
	out := make(map[string]middleware.Middleware, len(Registry.middlewares))
	for k, v := range Registry.middlewares {
		out[k] = v
	}
	return out
}

// RegisterEventListener 注册事件监听器。若 DefaultBus 非空则订阅该总线，否则在首次获得 DefaultBus 时再订阅（延迟订阅）。
// async 为 true 时在独立 goroutine 中调用，不阻塞发布方。
func RegisterEventListener(async bool, l events.Listener) {
	Registry.mu.Lock()
	defer Registry.mu.Unlock()
	Registry.listeners = append(Registry.listeners, listenerEntry{async: async, l: l})
	if bus := events.DefaultBus(); bus != nil {
		bus.Subscribe(async, l)
	}
}

// ApplyListenersTo 将已注册的监听器全部订阅到给定总线（用于主程序设置 DefaultBus 后统一挂载）。
func ApplyListenersTo(bus *events.Bus) {
	if bus == nil {
		return
	}
	Registry.mu.RLock()
	entries := make([]listenerEntry, len(Registry.listeners))
	copy(entries, Registry.listeners)
	Registry.mu.RUnlock()
	for _, e := range entries {
		bus.Subscribe(e.async, e.l)
	}
}
