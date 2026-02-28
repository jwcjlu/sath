package dsl

import (
	"context"
	"fmt"
	"strings"

	"github.com/sath/metadata"
)

var writePrefixes = []string{"insert ", "update ", "delete ", "replace ", "create ", "alter ", "drop "}

func isWriteDSL(dsl string) bool {
	upper := strings.TrimSpace(strings.ToLower(dsl))
	for _, p := range writePrefixes {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}

// MySQLValidator 基于关键字与元数据的简单校验，不连接数据库。
type MySQLValidator struct{}

// Validate 实现 Validator。
func (v *MySQLValidator) Validate(ctx context.Context, dsl string, meta *metadata.Schema, readOnly bool) error {
	dsl = strings.TrimSpace(dsl)
	if dsl == "" {
		return fmt.Errorf("dsl: empty")
	}
	if readOnly && isWriteDSL(dsl) {
		return fmt.Errorf("dsl: write not allowed in read-only mode")
	}
	// 简单语义：SELECT 中出现的表名应在 meta 中（可选，首版仅做写操作拒绝）
	// 可扩展：解析 SQL 提取表名并与 meta 比对
	_ = meta
	return nil
}
