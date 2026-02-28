package parser

import (
	"testing"
)

func TestJSONParser_Strict(t *testing.T) {
	p := NewJSONParser(true)
	var out struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	text := `{"name":"alice","age":30}`
	if err := p.Parse(text, &out); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if out.Name != "alice" || out.Age != 30 {
		t.Fatalf("unexpected result: %#v", out)
	}
}

func TestJSONParser_Lenient(t *testing.T) {
	p := NewJSONParser(false)
	var out map[string]any
	text := "some explanation...\n```json\n{\"k\":1}\n```"
	if err := p.Parse(text, &out); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if out["k"] != float64(1) {
		t.Fatalf("unexpected result: %#v", out)
	}
}

func TestKVParser_Basic(t *testing.T) {
	p := NewKVParser()
	var out map[string]string
	text := `
name: alice
age: 30
invalid line
city:  shanghai
`
	if err := p.Parse(text, &out); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(out))
	}
	if out["name"] != "alice" || out["age"] != "30" || out["city"] != "shanghai" {
		t.Fatalf("unexpected result: %#v", out)
	}
}
