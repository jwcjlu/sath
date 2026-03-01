package tool

import (
	"context"
	"testing"

	"github.com/sath/metadata"
)

func TestRegisterListTablesTool_AndExecute(t *testing.T) {
	reg := NewRegistry()
	store := metadata.NewInMemoryStore(func(ctx context.Context) (*metadata.Schema, error) {
		return &metadata.Schema{
			Name: "testdb",
			Tables: []metadata.Table{
				{Name: "users", Comment: "用户表"},
				{Name: "orders", Comment: ""},
			},
		}, nil
	})
	cfg := &ListTablesConfig{
		Store:               store,
		Registry:            nil,
		DefaultDatasourceID: "default",
	}
	err := RegisterListTablesTool(reg, cfg)
	if err != nil {
		t.Fatalf("RegisterListTablesTool: %v", err)
	}

	tool, ok := reg.Get("list_tables")
	if !ok {
		t.Fatal("list_tables not found")
	}
	if tool.Name != "list_tables" {
		t.Errorf("name: %s", tool.Name)
	}
	if tool.Parameters["type"] != "object" {
		t.Errorf("parameters type: %v", tool.Parameters["type"])
	}

	ctx := context.Background()
	_, _ = store.Refresh(ctx)

	out, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	list, ok := ListTablesResult(out)
	if !ok {
		t.Fatalf("ListTablesResult: %T %v", out, out)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(list))
	}
	if list[0]["name"] != "users" || list[0]["comment"] != "用户表" {
		t.Errorf("first table: %v", list[0])
	}
	if list[1]["name"] != "orders" {
		t.Errorf("second table: %v", list[1])
	}
}

func TestListTables_UseParamsDatasourceId(t *testing.T) {
	store := metadata.NewInMemoryStore(func(ctx context.Context) (*metadata.Schema, error) {
		return &metadata.Schema{
			Name:   "db2",
			Tables: []metadata.Table{{Name: "t1"}},
		}, nil
	})
	cfg := &ListTablesConfig{
		Store:               store,
		DefaultDatasourceID: "default",
	}
	reg := NewRegistry()
	_ = RegisterListTablesTool(reg, cfg)
	_, _ = store.Refresh(context.Background())

	tool, _ := reg.Get("list_tables")
	out, err := tool.Execute(context.Background(), map[string]any{"datasource_id": "other"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	list, ok := ListTablesResult(out)
	if !ok || len(list) != 1 || list[0]["name"] != "t1" {
		t.Errorf("out: %v ok=%v", out, ok)
	}
}

func TestListTables_NotConfigured(t *testing.T) {
	reg := NewRegistry()
	err := RegisterListTablesTool(reg, nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	tool, _ := reg.Get("list_tables")
	_, err = tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error when not configured")
	}
	if err.Error() != "list_tables: not configured (missing store)" {
		t.Errorf("error: %v", err)
	}
}

func TestListTables_DatasourceIdRequired(t *testing.T) {
	store := metadata.NewInMemoryStore(nil)
	cfg := &ListTablesConfig{
		Store:               store,
		DefaultDatasourceID: "",
	}
	reg := NewRegistry()
	_ = RegisterListTablesTool(reg, cfg)
	tool, _ := reg.Get("list_tables")
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error when datasource_id missing and no default")
	}
	if err.Error() != "list_tables: datasource_id is required (or set default)" {
		t.Errorf("error: %v", err)
	}
}

func TestListTables_RefreshWhenNoCache(t *testing.T) {
	// Store 无缓存且未提供 Registry 时，Execute 返回 "no schema available"。
	store := metadata.NewInMemoryStore(nil)
	cfg := &ListTablesConfig{
		Store:               store,
		Registry:            nil,
		DefaultDatasourceID: "ds1",
	}
	reg := NewRegistry()
	_ = RegisterListTablesTool(reg, cfg)
	tool, _ := reg.Get("list_tables")
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error when store has no cache and no registry")
	}
}
