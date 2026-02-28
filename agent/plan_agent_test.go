package agent

import (
	"context"
	"testing"

	"github.com/sath/model"
)

type stubPlanner struct {
	lastPrompt string
	reply      string
}

func (s *stubPlanner) Generate(ctx context.Context, prompt string, opts ...model.Option) (*model.Generation, error) {
	_ = ctx
	_ = opts
	s.lastPrompt = prompt
	return &model.Generation{Text: s.reply}, nil
}

func (s *stubPlanner) Chat(ctx context.Context, messages []model.Message, opts ...model.Option) (*model.Generation, error) {
	return &model.Generation{Text: "unused"}, nil
}

func (s *stubPlanner) Embed(ctx context.Context, texts []string, opts ...model.Option) ([]model.Embedding, error) {
	return nil, nil
}

type stubWorker struct {
	lastReq *Request
	reply   *Response
}

func (w *stubWorker) Run(ctx context.Context, req *Request) (*Response, error) {
	_ = ctx
	w.lastReq = req
	return w.reply, nil
}

func TestPlanExecuteAgent_Run(t *testing.T) {
	planner := &stubPlanner{reply: "step1, step2"}
	worker := &stubWorker{reply: &Response{Text: "done"}}

	agent := NewPlanExecuteAgent(planner, worker)

	req := &Request{
		Messages: []model.Message{
			{Role: "user", Content: "task A"},
			{Role: "assistant", Content: "ok"},
			{Role: "user", Content: "task B"},
		},
	}

	resp, err := agent.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if resp == nil || resp.Text != "done" {
		t.Fatalf("unexpected response: %#v", resp)
	}

	if planner.lastPrompt == "" {
		t.Fatalf("expected planner to be called")
	}
	if worker.lastReq == nil {
		t.Fatalf("expected worker to receive request")
	}
	if worker.lastReq.Metadata == nil || worker.lastReq.Metadata["plan"] != "step1, step2" {
		t.Fatalf("expected plan in metadata, got %#v", worker.lastReq.Metadata)
	}
}
