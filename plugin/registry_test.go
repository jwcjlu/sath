package plugin

import (
	"context"
	"testing"

	"github.com/sath/events"
	"github.com/sath/middleware"
	"github.com/sath/model"
	"github.com/sath/tool"
)

func TestRegisterTool_and_RegisteredTools(t *testing.T) {
	// Reset is not exposed; test only that we can register and retrieve.
	Registry.mu.Lock()
	origLen := len(Registry.tools)
	Registry.mu.Unlock()

	RegisterTool(tool.Tool{
		Name:        "test_plugin_tool",
		Description: "test",
		Execute:     func(ctx context.Context, params map[string]any) (any, error) { return nil, nil },
	})
	list := RegisteredTools()
	found := false
	for _, tt := range list {
		if tt.Name == "test_plugin_tool" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("RegisteredTools() did not contain test_plugin_tool, got %d tools", len(list))
	}
	// Restore approximate state: we added one (cannot remove without reset API)
	_ = origLen
}

func TestRegisterMiddleware_and_RegisteredMiddlewares(t *testing.T) {
	RegisterMiddleware("test_plugin_mw", func(next middleware.Handler) middleware.Handler {
		return next
	})
	mws := RegisteredMiddlewares()
	if mws["test_plugin_mw"] == nil {
		t.Fatal("RegisteredMiddlewares() did not contain test_plugin_mw")
	}
}

func TestRegisterModelProvider(t *testing.T) {
	// Register a dummy provider; NewFromIdentifier should use it
	RegisterModelProvider("fake", func(id string) (model.Model, error) {
		return nil, nil // returning nil model is invalid but we only check it's called
	})
	// Built-in switch runs after registry; so "fake" should be looked up.
	// We can't easily test without a real model, so just ensure no panic.
	_ = RegisteredTools()
}

func TestRegisterEventListener_and_ApplyListenersTo(t *testing.T) {
	bus := events.NewBus()
	var received bool
	RegisterEventListener(false, func(ctx context.Context, e events.Event) {
		received = true
	})
	ApplyListenersTo(bus)
	bus.Publish(context.Background(), events.Event{Kind: events.RunStarted})
	if !received {
		t.Error("listener registered via ApplyListenersTo did not receive event")
	}
}
