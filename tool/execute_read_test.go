package tool

import (
	"context"
	"errors"
	"testing"

	"github.com/sath/executor"
)

type fakeExecutor struct {
	calls []execCall
	ret   *executor.Result
	err   error
}

type execCall struct {
	DatasourceID string
	DSL          string
	Opts         executor.ExecuteOptions
}

func (f *fakeExecutor) Execute(ctx context.Context, datasourceID, dsl string, opts executor.ExecuteOptions) (*executor.Result, error) {
	f.calls = append(f.calls, execCall{
		DatasourceID: datasourceID,
		DSL:          dsl,
		Opts:         opts,
	})
	return f.ret, f.err
}

func TestExecuteRead_Basic(t *testing.T) {
	f := &fakeExecutor{
		ret: &executor.Result{
			Columns: []string{"id"},
			Rows:    [][]any{{int64(1)}},
		},
	}
	cfg := &ExecuteReadConfig{
		Exec:                f,
		DefaultDatasourceID: "ds1",
		DefaultTimeoutSec:   10,
		DefaultMaxRows:      100,
	}

	reg := NewRegistry()
	if err := RegisterExecuteReadTool(reg, cfg); err != nil {
		t.Fatalf("RegisterExecuteReadTool: %v", err)
	}
	tool, ok := reg.Get("execute_read")
	if !ok {
		t.Fatal("execute_read not found")
	}

	out, err := tool.Execute(context.Background(), map[string]any{
		"dsl": "SELECT 1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	res, ok := out.(*executor.Result)
	if !ok {
		t.Fatalf("unexpected result type: %T", out)
	}
	if len(res.Columns) != 1 || res.Columns[0] != "id" {
		t.Errorf("columns: %v", res.Columns)
	}
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.calls))
	}
	call := f.calls[0]
	if call.DatasourceID != "ds1" {
		t.Errorf("datasourceID: %s", call.DatasourceID)
	}
	if call.DSL != "SELECT 1" {
		t.Errorf("dsl: %s", call.DSL)
	}
	if !call.Opts.ReadOnly {
		t.Errorf("ReadOnly should be true")
	}
	if call.Opts.Timeout != 10 || call.Opts.MaxRows != 100 {
		t.Errorf("opts: %+v", call.Opts)
	}
}

func TestExecuteRead_OverrideOptionsAndDatasource(t *testing.T) {
	f := &fakeExecutor{ret: &executor.Result{}}
	cfg := &ExecuteReadConfig{
		Exec:                f,
		DefaultDatasourceID: "default",
		DefaultTimeoutSec:   0,
		DefaultMaxRows:      0,
	}
	reg := NewRegistry()
	_ = RegisterExecuteReadTool(reg, cfg)
	tool, _ := reg.Get("execute_read")

	_, err := tool.Execute(context.Background(), map[string]any{
		"query":         "SELECT * FROM t",
		"datasource_id": "other",
		"timeout_sec":   5,
		"max_rows":      2,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(f.calls))
	}
	call := f.calls[0]
	if call.DatasourceID != "other" {
		t.Errorf("datasourceID: %s", call.DatasourceID)
	}
	if call.DSL != "SELECT * FROM t" {
		t.Errorf("dsl: %s", call.DSL)
	}
	if call.Opts.Timeout != 5 || call.Opts.MaxRows != 2 {
		t.Errorf("opts: %+v", call.Opts)
	}
}

func TestExecuteRead_NotConfigured(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterExecuteReadTool(reg, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	tool, _ := reg.Get("execute_read")
	_, err := tool.Execute(context.Background(), map[string]any{
		"dsl": "SELECT 1",
	})
	if err == nil {
		t.Fatal("expected error when not configured")
	}
}

func TestExecuteRead_DatasourceRequired(t *testing.T) {
	f := &fakeExecutor{}
	cfg := &ExecuteReadConfig{
		Exec: f,
	}
	reg := NewRegistry()
	_ = RegisterExecuteReadTool(reg, cfg)
	tool, _ := reg.Get("execute_read")
	_, err := tool.Execute(context.Background(), map[string]any{
		"dsl": "SELECT 1",
	})
	if err == nil {
		t.Fatal("expected error when datasource_id missing and no default")
	}
}

func TestExecuteRead_DslRequired(t *testing.T) {
	f := &fakeExecutor{}
	cfg := &ExecuteReadConfig{
		Exec:                f,
		DefaultDatasourceID: "ds1",
	}
	reg := NewRegistry()
	_ = RegisterExecuteReadTool(reg, cfg)
	tool, _ := reg.Get("execute_read")
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error when dsl/query missing")
	}
}

func TestExecuteRead_InvalidTimeoutOrMaxRows(t *testing.T) {
	f := &fakeExecutor{}
	cfg := &ExecuteReadConfig{
		Exec:                f,
		DefaultDatasourceID: "ds1",
	}
	reg := NewRegistry()
	_ = RegisterExecuteReadTool(reg, cfg)
	tool, _ := reg.Get("execute_read")

	if _, err := tool.Execute(context.Background(), map[string]any{
		"dsl":         "SELECT 1",
		"timeout_sec": -1,
	}); err == nil {
		t.Fatal("expected error for negative timeout_sec")
	}
	if _, err := tool.Execute(context.Background(), map[string]any{
		"dsl":      "SELECT 1",
		"max_rows": -2,
	}); err == nil {
		t.Fatal("expected error for negative max_rows")
	}
}

func TestExecuteRead_ExecutorErrorWrapped(t *testing.T) {
	inner := errors.New("boom")
	f := &fakeExecutor{
		err: inner,
	}
	cfg := &ExecuteReadConfig{
		Exec:                f,
		DefaultDatasourceID: "ds1",
	}
	reg := NewRegistry()
	_ = RegisterExecuteReadTool(reg, cfg)
	tool, _ := reg.Get("execute_read")

	_, err := tool.Execute(context.Background(), map[string]any{
		"dsl": "SELECT 1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, inner) {
		t.Fatalf("expected wrapped inner error, got: %v", err)
	}
}
