package metadata

import (
	"context"
	"errors"
	"testing"
)

func TestInMemoryStore_GetSchema_Empty(t *testing.T) {
	store := NewInMemoryStore(nil)
	schema, err := store.GetSchema(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema != nil {
		t.Fatalf("expected nil schema, got %+v", schema)
	}
}

func TestInMemoryStore_Refresh_And_Get(t *testing.T) {
	called := false
	store := NewInMemoryStore(func(ctx context.Context) (*Schema, error) {
		called = true
		return &Schema{
			Name: "testdb",
			Tables: []Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "id", Type: "int"},
					},
				},
			},
		}, nil
	})

	ctx := context.Background()
	s1, err := store.Refresh(ctx)
	if err != nil {
		t.Fatalf("Refresh error: %v", err)
	}
	if !called {
		t.Fatalf("expected fetch to be called")
	}
	if s1 == nil || s1.Name != "testdb" {
		t.Fatalf("unexpected schema after refresh: %+v", s1)
	}

	s2, err := store.GetSchema(ctx)
	if err != nil {
		t.Fatalf("GetSchema error: %v", err)
	}
	if s2 == nil || s2.Name != "testdb" {
		t.Fatalf("unexpected schema from cache: %+v", s2)
	}

	// 修改返回值不应影响缓存
	s1.Name = "other"
	s3, err := store.GetSchema(ctx)
	if err != nil {
		t.Fatalf("GetSchema error: %v", err)
	}
	if s3.Name != "testdb" {
		t.Fatalf("schema should be copied on read, got: %+v", s3)
	}
}

func TestInMemoryStore_Refresh_Error(t *testing.T) {
	expErr := errors.New("boom")
	store := NewInMemoryStore(func(ctx context.Context) (*Schema, error) {
		return nil, expErr
	})
	_, err := store.Refresh(context.Background())
	if !errors.Is(err, expErr) {
		t.Fatalf("expected error %v, got %v", expErr, err)
	}
}

func TestInMemoryStore_GetTable(t *testing.T) {
	store := NewInMemoryStore(func(ctx context.Context) (*Schema, error) {
		return &Schema{
			Name: "testdb",
			Tables: []Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "id", Type: "int"},
					},
				},
			},
		}, nil
	})
	if _, err := store.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh error: %v", err)
	}

	ctx := context.Background()
	tbl, err := store.GetTable(ctx, "users")
	if err != nil {
		t.Fatalf("GetTable error: %v", err)
	}
	if tbl == nil || tbl.Name != "users" {
		t.Fatalf("unexpected table: %+v", tbl)
	}

	none, err := store.GetTable(ctx, "not_exists")
	if err != nil {
		t.Fatalf("GetTable error: %v", err)
	}
	if none != nil {
		t.Fatalf("expected nil for not found table, got: %+v", none)
	}
}
