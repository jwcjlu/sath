package datasource

import (
	"errors"
	"sync"
)

var (
	ErrUnknownType = errors.New("datasource: unknown type")
	ErrNotFound    = errors.New("datasource: not found")
)

// Factory 根据 Config 创建 DataSource，由各驱动在 init 或显式注册时注册。
type Factory func(cfg Config) (DataSource, error)

// Registry 管理已注册的 DataSource 与类型工厂。
type Registry struct {
	mu       sync.RWMutex
	sources  map[string]DataSource
	factories map[string]Factory
}

// NewRegistry 返回新的空 Registry。
func NewRegistry() *Registry {
	return &Registry{
		sources:   make(map[string]DataSource),
		factories: make(map[string]Factory),
	}
}

// RegisterType 注册某类型的工厂，后续 Register(cfg) 时若 cfg.Type 匹配则调用该工厂。
func (r *Registry) RegisterType(typ string, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factories == nil {
		r.factories = make(map[string]Factory)
	}
	r.factories[typ] = f
}

// Register 根据 cfg 创建并注册一个 DataSource。若该类型未注册工厂则返回 ErrUnknownType。
func (r *Registry) Register(cfg Config) (DataSource, error) {
	if cfg.ID == "" {
		return nil, ErrUnknownType
	}
	r.mu.Lock()
	f := r.factories[cfg.Type]
	r.mu.Unlock()
	if f == nil {
		return nil, ErrUnknownType
	}
	ds, err := f(cfg)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sources == nil {
		r.sources = make(map[string]DataSource)
	}
	r.sources[cfg.ID] = ds
	return ds, nil
}

// Get 按 ID 返回已注册的 DataSource，不存在返回 ErrNotFound。
func (r *Registry) Get(id string) (DataSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ds, ok := r.sources[id]
	if !ok {
		return nil, ErrNotFound
	}
	return ds, nil
}

// List 返回当前已注册的所有 DataSource（副本）。
func (r *Registry) List() []DataSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]DataSource, 0, len(r.sources))
	for _, ds := range r.sources {
		out = append(out, ds)
	}
	return out
}
