package dsl

import (
	"context"

	"github.com/sath/intent"
	"github.com/sath/metadata"
)

// Generator 根据意图与元数据生成可执行 DSL（如 SQL）及参数。
type Generator interface {
	Generate(ctx context.Context, input *intent.ParsedInput, meta *metadata.Schema) (dsl string, params []any, err error)
}
