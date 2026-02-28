package tool

import (
	"context"
	"encoding/json"
	"errors"
)

// RegisterCalculatorTool 向注册表中注册一个简单的加法计算工具。
func RegisterCalculatorTool(r *Registry) error {
	return r.Register(Tool{
		Name:        "calculator_add",
		Description: "Add two numbers and return the sum",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
		Execute: func(ctx context.Context, params map[string]any) (any, error) {
			a, okA := toFloat(params["a"])
			b, okB := toFloat(params["b"])
			if !okA || !okB {
				return nil, errors.New("invalid parameters for calculator_add")
			}
			_ = ctx
			return a + b, nil
		},
	})
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
