package errs

import "errors"

// 统一错类型，便于上层区分业务错误与系统错误，并决定是否重试、如何向外暴露。

var (
	// ErrBadRequest 表示调用方请求非法（参数缺失/格式错误等）。
	ErrBadRequest = errors.New("bad request")
	// ErrUnauthorized 表示鉴权失败或未提供凭证。
	ErrUnauthorized = errors.New("unauthorized")
	// ErrRateLimited 表示被限流。
	ErrRateLimited = errors.New("rate limited")
	// ErrContentBlocked 表示被内容安全策略拦截。
	ErrContentBlocked = errors.New("content blocked")
	// ErrInternal 表示内部未知错误或 panic 恢复。
	ErrInternal = errors.New("internal error")
)
