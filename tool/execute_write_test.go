package tool

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sath/auth"
	"github.com/sath/events"
	"github.com/sath/executor"
)

type fakeTokenGen struct {
	next string
	err  error
}

func (f *fakeTokenGen) NewToken() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.next, nil
}

type memoryPendingStore struct {
	items map[string]PendingWrite
}

func newMemoryPendingStore() *memoryPendingStore {
	return &memoryPendingStore{
		items: make(map[string]PendingWrite),
	}
}

func (m *memoryPendingStore) key(sessionID, token string) string {
	return sessionID + ":" + token
}

func (m *memoryPendingStore) SavePending(ctx context.Context, sessionID string, p PendingWrite) error {
	m.items[m.key(sessionID, p.Token)] = p
	return nil
}

func (m *memoryPendingStore) GetPending(ctx context.Context, sessionID, token string) (*PendingWrite, error) {
	if p, ok := m.items[m.key(sessionID, token)]; ok {
		cp := p
		return &cp, nil
	}
	return nil, nil
}

func (m *memoryPendingStore) DeletePending(ctx context.Context, sessionID, token string) error {
	delete(m.items, m.key(sessionID, token))
	return nil
}

type fakeChecker struct {
	allow bool
	calls int
}

func (f *fakeChecker) CanQuery(ctx context.Context, userID, datasourceID, dsl string) bool {
	return true
}

func (f *fakeChecker) CanExecute(ctx context.Context, userID, datasourceID, dsl string) bool {
	f.calls++
	return f.allow
}

type fakeWriteExecutor struct {
	calls []execCall
	ret   *executor.Result
	err   error
}

func (f *fakeWriteExecutor) Execute(ctx context.Context, datasourceID, dsl string, opts executor.ExecuteOptions) (*executor.Result, error) {
	f.calls = append(f.calls, execCall{
		DatasourceID: datasourceID,
		DSL:          dsl,
		Opts:         opts,
	})
	return f.ret, f.err
}

func TestExecuteWrite_ProposeAndConfirm_Success(t *testing.T) {
	// 设置审计事件总线，收集事件用于断言。
	bus := events.NewBus()
	var got []events.Event
	bus.Subscribe(false, func(ctx context.Context, e events.Event) {
		got = append(got, e)
	})
	events.SetDefaultBus(bus)

	store := newMemoryPendingStore()
	tokenGen := &fakeTokenGen{next: "t-123"}
	checker := &fakeChecker{allow: true}
	ex := &fakeWriteExecutor{
		ret: &executor.Result{AffectedRows: 1},
	}
	cfg := &ExecuteWriteConfig{
		Exec:                ex,
		Checker:             checker,
		PendingStore:        store,
		TokenGen:            tokenGen,
		DefaultDatasourceID: "ds1",
		ConfirmTTLSeconds:   300,
		DefaultTimeoutSec:   30,
	}

	reg := NewRegistry()
	if err := RegisterExecuteWriteTool(reg, cfg); err != nil {
		t.Fatalf("RegisterExecuteWriteTool: %v", err)
	}
	tool, _ := reg.Get("execute_write")

	// 阶段一：提议写/改
	ctx := context.WithValue(context.Background(), "request_id", "req-1")

	respAny, err := tool.Execute(ctx, map[string]any{
		"dsl":        "UPDATE users SET name = 'x' WHERE id = 1",
		"session_id": "s1",
		"user_id":    "u1",
	})
	if err != nil {
		t.Fatalf("propose Execute: %v", err)
	}
	resp, ok := respAny.(ExecuteWritePendingResponse)
	if !ok {
		t.Fatalf("unexpected response type: %T", respAny)
	}
	if resp.Status != "pending" || resp.Token != "t-123" {
		t.Errorf("pending resp: %+v", resp)
	}
	if checker.calls != 1 {
		t.Errorf("expected CanExecute called once, got %d", checker.calls)
	}

	// 阶段二：确认执行
	out, err := tool.Execute(ctx, map[string]any{
		"dsl":           "ignored by confirm path",
		"session_id":    "s1",
		"confirm_token": "t-123",
	})
	if err != nil {
		t.Fatalf("confirm Execute: %v", err)
	}
	res, ok := out.(*executor.Result)
	if !ok {
		t.Fatalf("unexpected result type: %T", out)
	}
	if res.AffectedRows != 1 {
		t.Errorf("AffectedRows = %d, want 1", res.AffectedRows)
	}
	if len(ex.calls) != 1 {
		t.Fatalf("expected 1 executor call, got %d", len(ex.calls))
	}
	call := ex.calls[0]
	if call.DatasourceID != "ds1" {
		t.Errorf("datasourceID: %s", call.DatasourceID)
	}
	if call.DSL != "UPDATE users SET name = 'x' WHERE id = 1" {
		t.Errorf("dsl: %s", call.DSL)
	}
	if call.Opts.ReadOnly {
		t.Errorf("ReadOnly should be false")
	}

	// 再次使用同一 token 应失败（一次性生效）
	if _, err := tool.Execute(ctx, map[string]any{
		"dsl":           "ignored",
		"session_id":    "s1",
		"confirm_token": "t-123",
	}); err == nil {
		t.Fatal("expected error when reuse token")
	}

	// 断言审计事件：一次 proposed + 一次 executed，且包含关键字段与 RequestID。
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].Kind != events.DataQueryWriteProposed {
		t.Errorf("first event kind: %s", got[0].Kind)
	}
	if got[1].Kind != events.DataQueryWriteExecuted {
		t.Errorf("second event kind: %s", got[1].Kind)
	}
	if got[0].RequestID != "req-1" || got[1].RequestID != "req-1" {
		t.Errorf("request ids: %s, %s", got[0].RequestID, got[1].RequestID)
	}
	if got[0].Payload["user_id"] != "u1" || got[0].Payload["session_id"] != "s1" {
		t.Errorf("proposed payload: %+v", got[0].Payload)
	}
	if got[1].Payload["confirmed"] != true || got[1].Payload["success"] != true {
		t.Errorf("executed payload: %+v", got[1].Payload)
	}
}

func TestExecuteWrite_PermissionDenied(t *testing.T) {
	store := newMemoryPendingStore()
	tokenGen := &fakeTokenGen{next: "t-123"}
	checker := &fakeChecker{allow: false}
	cfg := &ExecuteWriteConfig{
		Exec:                &fakeWriteExecutor{},
		Checker:             checker,
		PendingStore:        store,
		TokenGen:            tokenGen,
		DefaultDatasourceID: "ds1",
	}
	reg := NewRegistry()
	_ = RegisterExecuteWriteTool(reg, cfg)
	tool, _ := reg.Get("execute_write")

	_, err := tool.Execute(context.Background(), map[string]any{
		"dsl":        "DELETE FROM users",
		"session_id": "s1",
		"user_id":    "u1",
	})
	if err == nil {
		t.Fatal("expected error when permission denied")
	}
	if checker.calls != 1 {
		t.Errorf("expected CanExecute called once, got %d", checker.calls)
	}
	if len(store.items) != 0 {
		t.Errorf("expected no pending stored on deny, got %v", store.items)
	}
}

func TestExecuteWrite_InvalidOrExpiredToken(t *testing.T) {
	store := newMemoryPendingStore()
	tokenGen := &fakeTokenGen{next: "t-123"}
	ex := &fakeWriteExecutor{}
	now := time.Now()
	// 预置一个已过期的 pending
	_ = store.SavePending(context.Background(), "s1", PendingWrite{
		Token:        "expired",
		DSL:          "UPDATE t SET a=1",
		DatasourceID: "ds1",
		UserID:       "u1",
		CreatedAt:    now.Add(-10 * time.Minute),
	})

	cfg := &ExecuteWriteConfig{
		Exec:                ex,
		PendingStore:        store,
		TokenGen:            tokenGen,
		DefaultDatasourceID: "ds1",
		ConfirmTTLSeconds:   60, // 1 分钟
	}
	reg := NewRegistry()
	_ = RegisterExecuteWriteTool(reg, cfg)
	tool, _ := reg.Get("execute_write")

	// 无效 token
	if _, err := tool.Execute(context.Background(), map[string]any{
		"dsl":           "ignored",
		"session_id":    "s1",
		"confirm_token": "no-such-token",
	}); err == nil {
		t.Fatal("expected error for invalid token")
	}

	// 过期 token
	if _, err := tool.Execute(context.Background(), map[string]any{
		"dsl":           "ignored",
		"session_id":    "s1",
		"confirm_token": "expired",
	}); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestExecuteWrite_NotConfigured(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterExecuteWriteTool(reg, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	tool, _ := reg.Get("execute_write")
	_, err := tool.Execute(context.Background(), map[string]any{
		"dsl":        "UPDATE t SET a=1",
		"session_id": "s1",
	})
	if err == nil {
		t.Fatal("expected error when not configured")
	}
}

func TestExecuteWrite_ValidateParams(t *testing.T) {
	store := newMemoryPendingStore()
	tokenGen := &fakeTokenGen{next: "t-123"}
	cfg := &ExecuteWriteConfig{
		Exec:                &fakeWriteExecutor{},
		PendingStore:        store,
		TokenGen:            tokenGen,
		DefaultDatasourceID: "",
	}
	reg := NewRegistry()
	_ = RegisterExecuteWriteTool(reg, cfg)
	tool, _ := reg.Get("execute_write")

	if _, err := tool.Execute(context.Background(), map[string]any{
		"dsl":        "UPDATE t SET a=1",
		"session_id": "s1",
	}); err == nil {
		t.Fatal("expected error when datasource_id missing and no default")
	}

	if _, err := tool.Execute(context.Background(), map[string]any{
		"dsl":           "",
		"session_id":    "s1",
		"datasource_id": "ds1",
	}); err == nil {
		t.Fatal("expected error when dsl is empty")
	}

	if _, err := tool.Execute(context.Background(), map[string]any{
		"dsl":           "UPDATE t SET a=1",
		"datasource_id": "ds1",
	}); err == nil {
		t.Fatal("expected error when session_id missing")
	}
}

func TestExecuteWrite_ExecutorErrorWrapped(t *testing.T) {
	store := newMemoryPendingStore()
	tokenGen := &fakeTokenGen{next: "t-123"}
	inner := errors.New("boom")
	ex := &fakeWriteExecutor{err: inner}
	cfg := &ExecuteWriteConfig{
		Exec:                ex,
		PendingStore:        store,
		TokenGen:            tokenGen,
		DefaultDatasourceID: "ds1",
	}
	reg := NewRegistry()
	_ = RegisterExecuteWriteTool(reg, cfg)
	tool, _ := reg.Get("execute_write")

	// 先写入一个 pending，以便直接走确认路径
	_ = store.SavePending(context.Background(), "s1", PendingWrite{
		Token:        "t-123",
		DSL:          "UPDATE t SET a=1",
		DatasourceID: "ds1",
		UserID:       "u1",
		CreatedAt:    time.Now(),
	})

	_, err := tool.Execute(context.Background(), map[string]any{
		"dsl":           "ignored",
		"session_id":    "s1",
		"confirm_token": "t-123",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, inner) {
		t.Fatalf("expected wrapped inner error, got: %v", err)
	}
}

// 确认 ExecuteWriteConfig.Checker 实现了 auth.Checker 接口的预期用法。
var _ auth.Checker = (*auth.PermissiveChecker)(nil)
