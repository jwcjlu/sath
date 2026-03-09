package datasource

import (
	"context"
	"testing"
)

func TestRegistry_Register_Get_List(t *testing.T) {
	r := NewRegistry()
	RegisterNoop(r)

	cfg := Config{ID: "noop-1", Type: "noop"}
	ds, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if ds.ID() != "noop-1" {
		t.Errorf("ID() = %s, want noop-1", ds.ID())
	}
	if err := ds.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}

	got, err := r.Get("noop-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != ds {
		t.Error("Get returned different instance")
	}

	list := r.List()
	if len(list) != 1 || list[0].ID() != "noop-1" {
		t.Errorf("List() = %v", list)
	}
}

func TestRegistry_Register_unknownType(t *testing.T) {
	r := NewRegistry()
	_, err := r.Register(Config{ID: "x", Type: "unknown"})
	if err != ErrUnknownType {
		t.Errorf("Register(unknown type) = %v, want ErrUnknownType", err)
	}
}

func TestRegistry_Get_notFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("missing")
	if err != ErrNotFound {
		t.Errorf("Get(missing) = %v, want ErrNotFound", err)
	}
}
