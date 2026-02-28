package tool

import (
	"context"
	"testing"
)

func BenchmarkRegistry_RegisterAndGet(b *testing.B) {
	t := Tool{
		Name:        "bench_tool",
		Description: "bench",
		Execute:     func(ctx context.Context, params map[string]any) (any, error) { return nil, nil },
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := NewRegistry()
		_ = r.Register(t)
		_, _ = r.Get("bench_tool")
	}
}

func BenchmarkRegistry_List(b *testing.B) {
	reg := NewRegistry()
	for i := 0; i < 10; i++ {
		_ = reg.Register(Tool{
			Name:        "tool_" + string(rune('a'+i)),
			Description: "d",
			Execute:     func(ctx context.Context, params map[string]any) (any, error) { return nil, nil },
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.List()
	}
}
