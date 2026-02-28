package parser

import (
	"testing"
)

func TestTableParser_ToSlice(t *testing.T) {
	p := NewTableParser()
	text := `
| A | B | C |
| 1 | 2 | 3 |
| 4 | 5 | 6 |
`
	var out [][]string
	if err := p.Parse(text, &out); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(out))
	}
	if out[0][0] != "A" || out[1][1] != "2" {
		t.Fatalf("unexpected cells: %#v", out)
	}
}

func TestTableParser_ToMapSlice(t *testing.T) {
	p := NewTableParser()
	text := "| name | age |\n| alice | 30 |\n| bob | 25 |"
	var out []map[string]string
	if err := p.Parse(text, &out); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 data rows, got %d", len(out))
	}
	if out[0]["name"] != "alice" || out[0]["age"] != "30" {
		t.Fatalf("unexpected row: %#v", out[0])
	}
}

func TestTableParser_InvalidDest(t *testing.T) {
	p := NewTableParser()
	var x int
	if err := p.Parse("|a|", &x); err != errUnsupportedTableDest {
		t.Fatalf("expected errUnsupportedTableDest, got %v", err)
	}
}
