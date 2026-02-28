package model

import (
	"context"
	"fmt"
)

// MultiModelStrategy 表示多模型选择策略（B.1.2）。
type MultiModelStrategy string

const (
	StrategyByName    MultiModelStrategy = "by_name"
	StrategyByCost    MultiModelStrategy = "by_cost"    // 按成本优先：从 costOrder 中选第一个可用
	StrategyByLatency MultiModelStrategy = "by_latency" // 按延迟优先：从 latencyOrder 中选第一个可用（后续可扩展为动态测速）
)

// MultiModel 将多份模型实现封装为一个 Model，按策略选择具体底层模型。
type MultiModel struct {
	defaultName  string
	models       map[string]Model
	strategy     MultiModelStrategy
	costOrder    []string // 成本从低到高，StrategyByCost 时使用
	latencyOrder []string // 延迟从低到高，StrategyByLatency 时选第一个可用
}

// NewMultiModelFromMap 使用已构造好的模型集合创建 MultiModel，仅支持 StrategyByName。
func NewMultiModelFromMap(defaultName string, models map[string]Model, strategy MultiModelStrategy) (*MultiModel, error) {
	return NewMultiModelWithOptions(MultiModelOptions{
		DefaultName: defaultName,
		Models:      models,
		Strategy:    strategy,
	})
}

// MultiModelOptions 创建 MultiModel 时的可选配置。
type MultiModelOptions struct {
	DefaultName  string
	Models       map[string]Model
	Strategy     MultiModelStrategy
	CostOrder    []string // 成本从低到高，StrategyByCost 时按此顺序选第一个可用
	LatencyOrder []string // 延迟从低到高，StrategyByLatency 时按此顺序选
}

// NewMultiModelWithOptions 按选项创建 MultiModel，支持 by_name / by_cost / by_latency。
func NewMultiModelWithOptions(opts MultiModelOptions) (*MultiModel, error) {
	if len(opts.Models) == 0 {
		return nil, fmt.Errorf("models is empty")
	}
	if _, ok := opts.Models[opts.DefaultName]; !ok {
		return nil, fmt.Errorf("default model %q not found", opts.DefaultName)
	}
	switch opts.Strategy {
	case StrategyByName, StrategyByCost, StrategyByLatency:
	default:
		return nil, fmt.Errorf("unsupported strategy: %s", opts.Strategy)
	}
	return &MultiModel{
		defaultName:  opts.DefaultName,
		models:       opts.Models,
		strategy:     opts.Strategy,
		costOrder:    opts.CostOrder,
		latencyOrder: opts.LatencyOrder,
	}, nil
}

func (m *MultiModel) selectModel(cfg *CallConfig) (Model, error) {
	var name string
	switch m.strategy {
	case StrategyByName:
		name = m.defaultName
		if cfg != nil && cfg.ModelName != "" {
			if _, ok := m.models[cfg.ModelName]; ok {
				name = cfg.ModelName
			} else {
				return nil, fmt.Errorf("model %q not found", cfg.ModelName)
			}
		}
	case StrategyByCost:
		if len(m.costOrder) > 0 {
			for _, n := range m.costOrder {
				if _, ok := m.models[n]; ok {
					name = n
					break
				}
			}
		}
		if name == "" {
			name = m.defaultName
		}
	case StrategyByLatency:
		if len(m.latencyOrder) > 0 {
			for _, n := range m.latencyOrder {
				if _, ok := m.models[n]; ok {
					name = n
					break
				}
			}
		}
		if name == "" {
			name = m.defaultName
		}
	default:
		name = m.defaultName
	}
	return m.models[name], nil
}

func (m *MultiModel) Generate(ctx context.Context, prompt string, opts ...Option) (*Generation, error) {
	cfg := ApplyOptions(opts...)
	backend, err := m.selectModel(cfg)
	if err != nil {
		return nil, err
	}
	return backend.Generate(ctx, prompt, opts...)
}

func (m *MultiModel) Chat(ctx context.Context, messages []Message, opts ...Option) (*Generation, error) {
	cfg := ApplyOptions(opts...)
	backend, err := m.selectModel(cfg)
	if err != nil {
		return nil, err
	}
	return backend.Chat(ctx, messages, opts...)
}

func (m *MultiModel) Embed(ctx context.Context, texts []string, opts ...Option) ([]Embedding, error) {
	cfg := ApplyOptions(opts...)
	backend, err := m.selectModel(cfg)
	if err != nil {
		return nil, err
	}
	return backend.Embed(ctx, texts, opts...)
}
