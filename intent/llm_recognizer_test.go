package intent

import (
	"context"
	"testing"

	"github.com/sath/model"
)

type fakeModel struct {
	reply string
}

func (f *fakeModel) Generate(ctx context.Context, prompt string, opts ...model.Option) (*model.Generation, error) {
	return &model.Generation{Text: f.reply}, nil
}
func (f *fakeModel) Chat(ctx context.Context, messages []model.Message, opts ...model.Option) (*model.Generation, error) {
	return &model.Generation{Text: f.reply}, nil
}
func (f *fakeModel) Embed(ctx context.Context, texts []string, opts ...model.Option) ([]model.Embedding, error) {
	return nil, nil
}

func TestLLMRecognizer_Recognize_fakeJSON(t *testing.T) {
	r := &LLMRecognizer{
		Model: &fakeModel{reply: `{"intent":"query","entities":{"table":"users","columns":["id","name"],"limit":5},"raw_nl":"查用户"}`},
	}
	ctx := context.Background()
	out, err := r.Recognize(ctx, "s1", []model.Message{{Role: "user", Content: "查用户"}}, nil)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if out.Intent != IntentQuery || out.Entities.Table != "users" || out.Entities.Limit != 5 {
		t.Errorf("got %+v", out)
	}
}

func TestLLMRecognizer_Recognize_withCodeBlock(t *testing.T) {
	r := &LLMRecognizer{
		Model: &fakeModel{reply: "```json\n{\"intent\":\"metadata\",\"entities\":{},\"raw_nl\":\"有哪些表\"}\n```"},
	}
	ctx := context.Background()
	out, err := r.Recognize(ctx, "s1", nil, nil)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if out.Intent != IntentMetadata {
		t.Errorf("got intent %s", out.Intent)
	}
}
