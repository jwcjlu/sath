package middleware

import (
	"context"
	"strings"

	"github.com/sath/agent"
	"github.com/sath/errs"
)

// ContentFilter 定义输入/输出内容检查的接口。
type ContentFilter interface {
	CheckInput(text string) error
	CheckOutput(text string) error
}

// SimpleBlocklistFilter 基于关键字黑名单的简单实现，用于演示与开发环境。
type SimpleBlocklistFilter struct {
	Blocked []string
}

func (f *SimpleBlocklistFilter) CheckInput(text string) error {
	for _, w := range f.Blocked {
		if w == "" {
			continue
		}
		if strings.Contains(text, w) {
			return errs.ErrContentBlocked
		}
	}
	return nil
}

func (f *SimpleBlocklistFilter) CheckOutput(text string) error {
	return f.CheckInput(text)
}

// ContentSafetyMiddleware 在请求前后进行内容安全检查（输入/输出）。
func ContentSafetyMiddleware(filter ContentFilter) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
			if filter == nil || req == nil {
				return next(ctx, req)
			}
			for _, m := range req.Messages {
				if m.Role == "user" {
					if err := filter.CheckInput(m.Content); err != nil {
						// 统一映射为 errs.ErrContentBlocked，避免向上层暴露具体命中词。
						return nil, errs.ErrContentBlocked
					}
				}
			}

			resp, err := next(ctx, req)
			if err != nil || resp == nil {
				return resp, err
			}

			if err := filter.CheckOutput(resp.Text); err != nil {
				return nil, errs.ErrContentBlocked
			}
			return resp, nil
		}
	}
}
