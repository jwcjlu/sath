package model

import (
	"fmt"
	"strings"
	"sync"
)

// ModelProvider 根据完整标识（如 "openai/gpt-4o"）创建模型实例，供插件等扩展注册。
type ModelProvider func(id string) (Model, error)

var (
	providerMu sync.RWMutex
	providers  = make(map[string]ModelProvider)
)

// RegisterProvider 注册一个模型 Provider，供 NewFromIdentifier 使用。
// 通常由插件在 init() 中调用。provider 名称为小写，如 "openai"、"ollama"。
func RegisterProvider(provider string, f ModelProvider) {
	if f == nil {
		return
	}
	providerMu.Lock()
	defer providerMu.Unlock()
	providers[strings.ToLower(provider)] = f
}

// NewFromIdentifier 根据配置标识创建模型实例。
// 约定格式类似： "openai/gpt-4o"、"openai/gpt-3.5-turbo"。
// 先查找插件注册的 Provider，再回退到内置的 openai/ollama。
func NewFromIdentifier(id string) (Model, error) {
	provider, modelName := parseModelIdentifier(id)

	providerMu.RLock()
	ext, ok := providers[provider]
	providerMu.RUnlock()
	if ok && ext != nil {
		return ext(id)
	}

	switch provider {
	case "openai", "":
		cli, err := NewOpenAIClient()
		if err != nil {
			return nil, err
		}
		if modelName != "" {
			cli.model = modelName
		}
		return cli, nil
	case "ollama":
		cli, err := NewOllamaClient()
		if err != nil {
			return nil, err
		}
		if modelName != "" {
			cli.model = modelName
		}
		return cli, nil
	case "dashscope":
		cli, err := NewDashScopeClient()
		if err != nil {
			return nil, err
		}
		if modelName != "" {
			cli.model = modelName
		}
		return cli, nil
	default:
		return nil, fmt.Errorf("unsupported model provider: %s", provider)
	}
}

func parseModelIdentifier(id string) (provider, modelName string) {
	if id == "" {
		return "", ""
	}
	parts := strings.SplitN(id, "/", 2)
	if len(parts) == 1 {
		return strings.ToLower(strings.TrimSpace(parts[0])), ""
	}
	return strings.ToLower(strings.TrimSpace(parts[0])), strings.TrimSpace(parts[1])
}
