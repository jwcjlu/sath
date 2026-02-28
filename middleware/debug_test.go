package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/sath/agent"
	"github.com/sath/model"
)

func TestDebugMiddleware_Disabled(t *testing.T) {
	next := func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		return &agent.Response{Text: "ok"}, nil
	}
	h := DebugMiddleware(false)(next)
	resp, err := h(context.Background(), &agent.Request{Messages: []model.Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || resp.Text != "ok" {
		t.Fatalf("unexpected resp: %#v", resp)
	}
}

func TestDebugMiddleware_Enabled_NoPanicOnNilRequest(t *testing.T) {
	next := func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		return nil, nil
	}
	h := DebugMiddleware(true)(next)
	resp, err := h(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp != nil {
		t.Fatalf("expected nil resp")
	}
}

func TestDebugMiddleware_Enabled_ErrorLogsStack(t *testing.T) {
	expectErr := errors.New("fail")
	next := func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		return nil, expectErr
	}
	h := DebugMiddleware(true)(next)
	resp, err := h(context.Background(), &agent.Request{RequestID: "r1", Messages: []model.Message{{Role: "user", Content: "x"}}})
	if err != expectErr {
		t.Fatalf("expected %v, got %v", expectErr, err)
	}
	if resp != nil {
		t.Fatalf("expected nil resp")
	}
}
