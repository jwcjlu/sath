package agent

import (
	"context"
	"testing"

	"github.com/sath/intent"
	"github.com/sath/metadata"
	"github.com/sath/model"
)

func TestDataQueryAgent_Run_noMessages(t *testing.T) {
	a := &DataQueryAgent{SessionStore: intent.NewInMemoryDataSessionStore()}
	ctx := context.Background()
	resp, err := a.Run(ctx, &Request{Messages: nil})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp == nil || resp.Text == "" {
		t.Error("expected non-empty prompt")
	}
}

func TestDataQueryAgent_Run_noSessionNoDefaultDS(t *testing.T) {
	a := &DataQueryAgent{
		SessionStore: intent.NewInMemoryDataSessionStore(),
		Config:       DataQueryConfig{},
	}
	ctx := context.Background()
	resp, err := a.Run(ctx, &Request{
		Messages: []model.Message{{Role: "user", Content: "查表"}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp != nil && resp.Text != "" {
		t.Logf("resp: %s", resp.Text)
	}
}

func TestDataQueryAgent_Run_metadataIntent(t *testing.T) {
	store := intent.NewInMemoryDataSessionStore()
	_ = store.Set("s1", &intent.DataSessionContext{DatasourceID: "ds1"})
	metaStore := metadata.NewInMemoryStore()
	_ = metaStore.Refresh(context.Background(), "ds1", func() (*metadata.Schema, error) {
		return &metadata.Schema{Name: "db", Tables: []*metadata.Table{{Name: "users", Columns: nil}}}, nil
	})
	a := &DataQueryAgent{
		Recognizer:   &fakeRecognizer{out: &intent.ParsedInput{Intent: intent.IntentMetadata}},
		SessionStore: store,
		MetaStore:    metaStore,
	}
	ctx := context.Background()
	req := &Request{
		Messages: []model.Message{{Role: "user", Content: "有哪些表"}},
		Metadata: map[string]any{"session_id": "s1"},
	}
	resp, err := a.Run(ctx, req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp == nil {
		t.Fatal("nil resp")
	}
	if resp.Text == "" {
		t.Error("expected metadata text")
	}
}

type fakeRecognizer struct {
	out *intent.ParsedInput
	err error
}

func (f *fakeRecognizer) Recognize(_ context.Context, _ string, _ []model.Message, _ *metadata.Schema) (*intent.ParsedInput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}
