package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/sath/agent"
	"github.com/sath/datasource"
	"github.com/sath/executor"
	"github.com/sath/memory"
	"github.com/sath/metadata"
	"github.com/sath/model"
	"github.com/sath/tool"
)

func TestBuildDataQuerySystemPrompt_ReadOnly(t *testing.T) {
	p := BuildDataQuerySystemPrompt(DataQueryPromptConfig{
		DatasourceType: "mysql",
		AllowWrite:     false,
	})
	if p == "" {
		t.Fatal("prompt should not be empty")
	}
	if !containsAll(p, []string{"mysql", "只读模式", "list_tables", "describe_table", "execute_read"}) {
		t.Fatalf("read-only prompt missing expected keywords:\n%s", p)
	}
	if contains(p, "execute_write：用于") {
		t.Fatalf("read-only prompt should not describe execute_write usage:\n%s", p)
	}
	if !contains(p, "禁用") {
		t.Fatalf("read-only prompt should mention execute_write is disabled:\n%s", p)
	}
}

func TestBuildDataQuerySystemPrompt_AllowWrite(t *testing.T) {
	p := BuildDataQuerySystemPrompt(DataQueryPromptConfig{
		DatasourceType: "mysql",
		AllowWrite:     true,
	})
	if p == "" {
		t.Fatal("prompt should not be empty")
	}
	if !containsAll(p, []string{
		"写/改", "execute_write", "两阶段确认", "确认 token",
		"写/改提议阶段", "写/改确认与执行阶段",
	}) {
		t.Fatalf("writable prompt missing expected phrases:\n%s", p)
	}
}

// fakeToolModel 实现 ToolCallingModel，用于验证 DataQueryReActAgent 的系统提示注入与工具注册。
type fakeToolModel struct {
	lastMessages []model.Message
}

func (f *fakeToolModel) Generate(ctx context.Context, prompt string, opts ...model.Option) (*model.Generation, error) {
	return &model.Generation{Text: ""}, nil
}

func (f *fakeToolModel) Chat(ctx context.Context, messages []model.Message, opts ...model.Option) (*model.Generation, error) {
	// 最后一轮回答文本。
	return &model.Generation{Text: "ok"}, nil
}

func (f *fakeToolModel) Embed(ctx context.Context, texts []string, opts ...model.Option) ([]model.Embedding, error) {
	return nil, nil
}

func (f *fakeToolModel) ChatWithTools(ctx context.Context, messages []model.Message, reg *tool.Registry, opts ...model.Option) (*model.Generation, error) {
	f.lastMessages = append([]model.Message(nil), messages...)
	// 验证工具已注册（这里不真正调用）。
	if _, ok := reg.Get("list_tables"); !ok {
		return nil, nil
	}
	if _, ok := reg.Get("describe_table"); !ok {
		return nil, nil
	}
	if _, ok := reg.Get("execute_read"); !ok {
		return nil, nil
	}
	// 返回一次「未使用工具」的步骤，让 ReActAgent 直接生成最终回答。
	return &model.Generation{
		Text: "tool step",
		Raw: model.ToolStep{
			Used: false,
		},
	}, nil
}

func TestNewDataQueryHandler_InjectsSystemPromptAndTools(t *testing.T) {
	m := &fakeToolModel{}
	mem := memory.NewBufferMemory(5)

	dsReg := datasource.NewRegistry()
	store := metadata.NewInMemoryStore(nil)
	h := NewDataQueryHandler(m, mem, DataQueryConfig{
		DatasourceRegistry:  dsReg,
		MetadataStore:       store,
		Exec:                executor.NewMySQLExecutor(dsReg), // 测试中不会真正访问 DB
		DefaultDatasourceID: "ds-default",
		AllowWrite:          false,
	} /* no middleware */)

	req := &agent.Request{
		Messages: []model.Message{
			{Role: "user", Content: "列出所有表"},
		},
		Metadata: map[string]any{
			"session_id":    "s-1",
			"user_id":       "u-1",
			"datasource_id": "ds-1",
		},
		RequestID: "req-xyz",
	}

	resp, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp == nil || resp.Text == "" {
		t.Fatalf("unexpected response: %#v", resp)
	}

	if len(m.lastMessages) == 0 || m.lastMessages[0].Role != "system" {
		t.Fatalf("first message should be system prompt, got: %#v", m.lastMessages)
	}
	sys := m.lastMessages[0].Content
	if !containsAll(sys, []string{"list_tables", "execute_read"}) {
		t.Fatalf("system prompt missing expected content:\n%s", sys)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
