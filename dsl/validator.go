package dsl

import (
	"context"

	"github.com/sath/metadata"
)

// Validator 对 DSL 做语法/语义校验；readOnly 为 true 时拒绝写操作。
type Validator interface {
	Validate(ctx context.Context, dsl string, meta *metadata.Schema, readOnly bool) error
}

// ConfirmRequest 执行前确认请求：展示给用户的 DSL 与描述，供用户确认后真正执行。
type ConfirmRequest struct {
	DSL          string `json:"dsl"`
	Description  string `json:"description,omitempty"`
	EstimateRows int    `json:"estimate_rows,omitempty"` // 0 表示未知
}

// ConfirmResponse 用户确认结果；Token 可用于下一轮请求携带以执行该 DSL。
type ConfirmResponse struct {
	Confirmed bool   `json:"confirmed"`
	Token     string `json:"token,omitempty"`
}
