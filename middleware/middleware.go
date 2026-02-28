package middleware

import (
	"context"
	"sort"

	"github.com/sath/agent"
)

// Handler 表示 Agent 的最终处理函数。
type Handler func(ctx context.Context, req *agent.Request) (*agent.Response, error)

// Middleware 用于包装 Handler，形成中间件链。
type Middleware func(Handler) Handler

// Chain 将多个中间件按顺序与最终 Handler 组合。
func Chain(final Handler, mws ...Middleware) Handler {
	if len(mws) == 0 {
		return final
	}
	h := final
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// OrderedMiddleware 带优先级的中间件，Order 越小越先执行（靠近 Handler）。
type OrderedMiddleware struct {
	Order int
	Mw    Middleware
}

// ChainBuilder 按 Order 排序后与 final 组合成 Handler（B.5.1 中间件顺序/优先级）。
func ChainBuilder(final Handler, ordered ...OrderedMiddleware) Handler {
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Order > ordered[j].Order })
	mws := make([]Middleware, len(ordered))
	for i := range ordered {
		mws[i] = ordered[i].Mw
	}
	return Chain(final, mws...)
}

// MergeGlobalLocal 将全局中间件与局部中间件合并：先执行全局，再执行局部，最后执行 final（B.5.2）。
// 即请求先经过 global...，再经过 local...，最后到达 core Handler。
func MergeGlobalLocal(final Handler, global, local []Middleware) Handler {
	all := make([]Middleware, 0, len(global)+len(local))
	all = append(all, global...)
	all = append(all, local...)
	return Chain(final, all...)
}
