package dsl

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMySQLValidator_Validate_readOnlyRejectsWrite(t *testing.T) {
	v := &MySQLValidator{}
	ctx := context.Background()
	err := v.Validate(ctx, "UPDATE users SET x=1", nil, true)
	if err == nil {
		t.Fatal("expected error for write in read-only")
	}
}

func TestMySQLValidator_Validate_selectAllowed(t *testing.T) {
	v := &MySQLValidator{}
	ctx := context.Background()
	err := v.Validate(ctx, "SELECT 1 FROM users", nil, true)
	if err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestConfirmRequest_Response_JSON(t *testing.T) {
	req := ConfirmRequest{DSL: "UPDATE t SET a=1", Description: "更新表 t", EstimateRows: 0}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out ConfirmRequest
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.DSL != req.DSL {
		t.Errorf("roundtrip: %+v", out)
	}
	resp := ConfirmResponse{Confirmed: true, Token: "tok123"}
	data2, _ := json.Marshal(resp)
	var out2 ConfirmResponse
	_ = json.Unmarshal(data2, &out2)
	if !out2.Confirmed || out2.Token != "tok123" {
		t.Errorf("roundtrip: %+v", out2)
	}
}

func TestMySQLValidator_Validate_empty(t *testing.T) {
	v := &MySQLValidator{}
	ctx := context.Background()
	err := v.Validate(ctx, "  ", metaWithUsers(), true)
	if err == nil {
		t.Fatal("expected error for empty dsl")
	}
}
