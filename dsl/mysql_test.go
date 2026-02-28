package dsl

import (
	"context"
	"strings"
	"testing"

	"github.com/sath/intent"
	"github.com/sath/metadata"
)

func metaWithUsers() *metadata.Schema {
	return &metadata.Schema{
		Name: "testdb",
		Tables: []*metadata.Table{
			{
				Name: "users",
				Columns: []*metadata.Column{
					{Name: "id", Type: "int"},
					{Name: "name", Type: "varchar(64)"},
					{Name: "status", Type: "varchar(32)"},
				},
			},
		},
	}
}

func TestMySQLGenerator_Generate_select(t *testing.T) {
	gen := &MySQLGenerator{}
	ctx := context.Background()
	meta := metaWithUsers()
	input := &intent.ParsedInput{
		Intent: intent.IntentQuery,
		Entities: intent.Entities{
			Table:   "users",
			Columns: []string{"id", "name"},
			Limit:   10,
		},
	}
	sql, params, err := gen.Generate(ctx, input, meta)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if sql != "SELECT `id`, `name` FROM `users` LIMIT ?" {
		t.Errorf("sql = %s", sql)
	}
	if len(params) != 1 || params[0] != 10 {
		t.Errorf("params = %v", params)
	}
}

func TestMySQLGenerator_Generate_selectWhere(t *testing.T) {
	gen := &MySQLGenerator{}
	ctx := context.Background()
	meta := metaWithUsers()
	input := &intent.ParsedInput{
		Intent: intent.IntentQuery,
		Entities: intent.Entities{
			Table: "users",
			Conditions: []intent.Condition{
				{Field: "status", Op: "eq", Value: "active"},
			},
		},
	}
	sql, params, err := gen.Generate(ctx, input, meta)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(sql, "WHERE") || !strings.Contains(sql, "`status` = ?") {
		t.Errorf("sql = %s", sql)
	}
	if len(params) != 1 || params[0] != "active" {
		t.Errorf("params = %v", params)
	}
}

func TestMySQLGenerator_Generate_insert(t *testing.T) {
	gen := &MySQLGenerator{}
	ctx := context.Background()
	meta := metaWithUsers()
	input := &intent.ParsedInput{
		Intent: intent.IntentInsert,
		Entities: intent.Entities{
			Table:  "users",
			Values: map[string]any{"name": "alice", "status": "active"},
		},
	}
	sql, params, err := gen.Generate(ctx, input, meta)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(sql, "INSERT INTO `users`") || !strings.Contains(sql, "VALUES") {
		t.Errorf("sql = %s", sql)
	}
	if len(params) != 2 {
		t.Errorf("params = %v", params)
	}
}

func TestMySQLGenerator_Generate_tableNotFound(t *testing.T) {
	gen := &MySQLGenerator{}
	ctx := context.Background()
	meta := metaWithUsers()
	input := &intent.ParsedInput{
		Intent: intent.IntentQuery,
		Entities: intent.Entities{Table: "notfound"},
	}
	_, _, err := gen.Generate(ctx, input, meta)
	if err == nil {
		t.Fatal("expected error for unknown table")
	}
}

