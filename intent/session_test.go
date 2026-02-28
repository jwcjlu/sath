package intent

import (
	"testing"
)

func TestInMemoryDataSessionStore_GetSet(t *testing.T) {
	s := NewInMemoryDataSessionStore()
	ctx := &DataSessionContext{DatasourceID: "ds1", LastTable: "users"}
	if err := s.Set("sid1", ctx); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get("sid1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DatasourceID != "ds1" || got.LastTable != "users" {
		t.Errorf("got %+v", got)
	}
	got2, _ := s.Get("missing")
	if got2 != nil {
		t.Errorf("Get(missing) should be nil, got %+v", got2)
	}
}

func TestGetDataContextFromMetadata(t *testing.T) {
	meta := map[string]any{"data_context": &DataSessionContext{LastDSL: "SELECT 1"}}
	c, err := GetDataContextFromMetadata(meta)
	if err != nil {
		t.Fatalf("GetDataContextFromMetadata: %v", err)
	}
	if c.LastDSL != "SELECT 1" {
		t.Errorf("got %+v", c)
	}
	SetDataContextInMetadata(meta, &DataSessionContext{DatasourceID: "ds2"})
	c2, _ := GetDataContextFromMetadata(meta)
	if c2.DatasourceID != "ds2" {
		t.Errorf("after set: %+v", c2)
	}
}
