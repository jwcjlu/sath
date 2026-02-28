package metadata

import (
	"context"
	"testing"
)

func TestInMemoryStore_GetSchema_empty(t *testing.T) {
	s := NewInMemoryStore()
	ctx := context.Background()
	sch, err := s.GetSchema(ctx, "ds1")
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	if sch != nil {
		t.Errorf("GetSchema expected nil, got %v", sch)
	}
}

func TestInMemoryStore_Refresh_and_GetSchema(t *testing.T) {
	s := NewInMemoryStore()
	ctx := context.Background()
	want := &Schema{
		Name: "testdb",
		Tables: []*Table{
			{Name: "users", Columns: []*Column{{Name: "id", Type: "INT", PrimaryKey: true}}},
		},
	}
	err := s.Refresh(ctx, "ds1", func() (*Schema, error) { return want, nil })
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	got, err := s.GetSchema(ctx, "ds1")
	if err != nil {
		t.Fatalf("GetSchema: %v", err)
	}
	if got != want {
		t.Errorf("GetSchema: got %p, want %p", got, want)
	}
	if got.Name != "testdb" || len(got.Tables) != 1 || got.Tables[0].Name != "users" {
		t.Errorf("GetSchema: got %+v", got)
	}
}

func TestInMemoryStore_Refresh_fetchError(t *testing.T) {
	s := NewInMemoryStore()
	ctx := context.Background()
	err := s.Refresh(ctx, "ds1", func() (*Schema, error) { return nil, context.DeadlineExceeded })
	if err != context.DeadlineExceeded {
		t.Errorf("Refresh: want DeadlineExceeded, got %v", err)
	}
	sch, _ := s.GetSchema(ctx, "ds1")
	if sch != nil {
		t.Error("GetSchema should still be nil after failed Refresh")
	}
}

func TestInMemoryStore_GetTable(t *testing.T) {
	s := NewInMemoryStore()
	ctx := context.Background()
	_ = s.Refresh(ctx, "ds1", func() (*Schema, error) {
		return &Schema{
			Name: "db",
			Tables: []*Table{
				{Name: "a", Columns: []*Column{{Name: "id"}}},
				{Name: "b", Columns: nil},
			},
		}, nil
	})
	tbl, err := s.GetTable(ctx, "ds1", "b")
	if err != nil {
		t.Fatalf("GetTable: %v", err)
	}
	if tbl == nil || tbl.Name != "b" {
		t.Errorf("GetTable: got %v", tbl)
	}
	tbl, _ = s.GetTable(ctx, "ds1", "missing")
	if tbl != nil {
		t.Errorf("GetTable(missing): got %v", tbl)
	}
}
