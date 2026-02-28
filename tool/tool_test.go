package tool

import (
	"context"
	"errors"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	tool := Tool{
		Name: "echo",
		Execute: func(ctx context.Context, params map[string]any) (any, error) {
			_ = ctx
			return params["value"], nil
		},
	}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	got, ok := r.Get("echo")
	if !ok {
		t.Fatalf("expected tool to be found")
	}
	if got.Name != "echo" {
		t.Fatalf("unexpected tool name: %s", got.Name)
	}

	resp, err := got.Execute(context.Background(), map[string]any{"value": "hello"})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if resp != "hello" {
		t.Fatalf("unexpected execute result: %v", resp)
	}
}

func TestRegistry_Register_Validation(t *testing.T) {
	r := NewRegistry()

	// 名称为空
	err := r.Register(Tool{
		Name:    "",
		Execute: func(context.Context, map[string]any) (any, error) { return nil, nil },
	})
	if err == nil {
		t.Fatalf("expected error for empty name")
	}

	// Execute 为空
	err = r.Register(Tool{
		Name: "no_exec",
	})
	if err == nil {
		t.Fatalf("expected error for nil execute")
	}
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := NewRegistry()

	tool := Tool{
		Name: "dup",
		Execute: func(context.Context, map[string]any) (any, error) {
			return nil, nil
		},
	}

	if err := r.Register(tool); err != nil {
		t.Fatalf("first Register error: %v", err)
	}
	err := r.Register(tool)
	if err == nil {
		t.Fatalf("expected duplicate register error")
	}
	if !errors.Is(err, err) { // 这里只是确保返回了非 nil 错误，具体文案不强绑定
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	tools := []Tool{
		{Name: "a", Execute: func(context.Context, map[string]any) (any, error) { return nil, nil }},
		{Name: "b", Execute: func(context.Context, map[string]any) (any, error) { return nil, nil }},
	}
	for _, tl := range tools {
		if err := r.Register(tl); err != nil {
			t.Fatalf("Register error: %v", err)
		}
	}

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(list))
	}
}
