package tool

import (
	"context"
	"testing"

	"github.com/sath/metadata"
)

func TestRegisterDescribeTableTool_AndExecute(t *testing.T) {
	reg := NewRegistry()
	store := metadata.NewInMemoryStore(func(ctx context.Context) (*metadata.Schema, error) {
		return &metadata.Schema{
			Name: "testdb",
			Tables: []metadata.Table{
				{
					Name:    "users",
					Comment: "用户表",
					Columns: []metadata.Column{
						{Name: "id", Type: "int", IsNullable: false},
						{Name: "name", Type: "varchar", IsNullable: true},
					},
				},
			},
		}, nil
	})
	cfg := &DescribeTableConfig{
		Store:               store,
		DefaultDatasourceID: "default",
	}

	if _, err := store.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if err := RegisterDescribeTableTool(reg, cfg); err != nil {
		t.Fatalf("RegisterDescribeTableTool: %v", err)
	}

	tool, ok := reg.Get("describe_table")
	if !ok {
		t.Fatal("describe_table not found")
	}
	if tool.Parameters["type"] != "object" {
		t.Errorf("parameters type: %v", tool.Parameters["type"])
	}

	out, err := tool.Execute(context.Background(), map[string]any{
		"table_name": "users",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	tbl, ok := out.(metadata.Table)
	if !ok {
		t.Fatalf("unexpected result type: %T", out)
	}
	if tbl.Name != "users" || tbl.Comment != "用户表" {
		t.Errorf("table: %+v", tbl)
	}
	if len(tbl.Columns) != 2 || tbl.Columns[0].Name != "id" || tbl.Columns[1].IsNullable != true {
		t.Errorf("columns: %+v", tbl.Columns)
	}
}

func TestDescribeTable_NotConfigured(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterDescribeTableTool(reg, nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	tool, _ := reg.Get("describe_table")
	_, err := tool.Execute(context.Background(), map[string]any{
		"table_name": "users",
	})
	if err == nil {
		t.Fatal("expected error when not configured")
	}
}

func TestDescribeTable_DatasourceIdRequired(t *testing.T) {
	store := metadata.NewInMemoryStore(nil)
	cfg := &DescribeTableConfig{
		Store:               store,
		DefaultDatasourceID: "",
	}
	reg := NewRegistry()
	_ = RegisterDescribeTableTool(reg, cfg)
	tool, _ := reg.Get("describe_table")
	_, err := tool.Execute(context.Background(), map[string]any{
		"table_name": "users",
	})
	if err == nil {
		t.Fatal("expected error when datasource_id missing and no default")
	}
}

func TestDescribeTable_TableNameRequired(t *testing.T) {
	store := metadata.NewInMemoryStore(nil)
	cfg := &DescribeTableConfig{
		Store:               store,
		DefaultDatasourceID: "ds1",
	}
	reg := NewRegistry()
	_ = RegisterDescribeTableTool(reg, cfg)
	tool, _ := reg.Get("describe_table")
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error when table_name missing")
	}
}

func TestDescribeTable_TableNotFound(t *testing.T) {
	store := metadata.NewInMemoryStore(func(ctx context.Context) (*metadata.Schema, error) {
		return &metadata.Schema{
			Name:   "db",
			Tables: []metadata.Table{{Name: "other"}},
		}, nil
	})
	_, _ = store.Refresh(context.Background())

	cfg := &DescribeTableConfig{
		Store:               store,
		DefaultDatasourceID: "ds1",
	}
	reg := NewRegistry()
	_ = RegisterDescribeTableTool(reg, cfg)
	tool, _ := reg.Get("describe_table")
	_, err := tool.Execute(context.Background(), map[string]any{
		"table_name": "not_exists",
	})
	if err == nil {
		t.Fatal("expected error when table not found")
	}
}
