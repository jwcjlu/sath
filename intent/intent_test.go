package intent

import (
	"encoding/json"
	"testing"
)

func TestParsedInput_JSON_roundtrip(t *testing.T) {
	in := &ParsedInput{
		Intent: IntentQuery,
		Entities: Entities{
			Table:   "users",
			Columns: []string{"id", "name"},
			Conditions: []Condition{
				{Field: "status", Op: "eq", Value: "active"},
			},
			Limit: 10,
		},
		RawNL: "查询活跃用户前10条",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out ParsedInput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Intent != in.Intent || out.Entities.Table != in.Entities.Table {
		t.Errorf("roundtrip: got %+v", out)
	}
	if len(out.Entities.Conditions) != 1 || out.Entities.Conditions[0].Field != "status" {
		t.Errorf("Conditions: %+v", out.Entities.Conditions)
	}
}

func TestParsedInput_insert_roundtrip(t *testing.T) {
	in := &ParsedInput{
		Intent: IntentInsert,
		Entities: Entities{
			Table: "users",
			Values: map[string]any{"name": "alice", "age": 20},
		},
		RawNL: "插入一条用户",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out ParsedInput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Intent != IntentInsert || out.Entities.Table != "users" {
		t.Errorf("roundtrip: %+v", out)
	}
}
