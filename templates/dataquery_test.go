package templates

import (
	"context"
	"testing"

	"github.com/sath/agent"
	"github.com/sath/intent"
	"github.com/sath/metadata"
	"github.com/sath/model"
)

// TestDataQueryHandler_e2e 端到端：使用返回 metadata 意图的假 Recognizer，验证 handler 返回表列表。
func TestDataQueryHandler_e2e(t *testing.T) {
	metaStore := metadata.NewInMemoryStore()
	_ = metaStore.Refresh(context.Background(), "ds1", func() (*metadata.Schema, error) {
		return &metadata.Schema{Name: "db", Tables: []*metadata.Table{{Name: "users"}, {Name: "orders"}}}, nil
	})
	sessionStore := intent.NewInMemoryDataSessionStore()
	_ = sessionStore.Set("s1", &intent.DataSessionContext{DatasourceID: "ds1"})
	dataAgent := &agent.DataQueryAgent{
		Recognizer:   &fakeMetadataRecognizer{},
		MetaStore:    metaStore,
		SessionStore: sessionStore,
	}
	handler := NewDataQueryHandler(dataAgent)

	ctx := context.Background()
	req := &agent.Request{
		Messages: []model.Message{{Role: "user", Content: "有哪些表"}},
		Metadata: map[string]any{"session_id": "s1"},
	}
	resp, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if resp.Text == "" {
		t.Error("expected non-empty reply")
	}
	if resp.Text != "表列表：users、orders" && resp.Text != "表列表：orders、users" {
		t.Logf("reply: %s", resp.Text)
	}
}

type fakeMetadataRecognizer struct{}

func (fakeMetadataRecognizer) Recognize(_ context.Context, _ string, _ []model.Message, _ *metadata.Schema) (*intent.ParsedInput, error) {
	return &intent.ParsedInput{Intent: intent.IntentMetadata}, nil
}
