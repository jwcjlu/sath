package agent

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/sath/dsl"
	"github.com/sath/errs"
	"github.com/sath/executor"
	"github.com/sath/intent"
	"github.com/sath/metadata"
)

// DataQueryConfig 控制 DataQueryAgent 的行为。
type DataQueryConfig struct {
	DefaultDatasourceID string
	ReadOnly            bool
	Timeout             time.Duration
	MaxRows             int
	ConfirmTimeoutSec   int64
}

// DataQueryAgent 编排意图识别 -> DSL 生成 -> 校验 -> 执行 -> 结果格式化，实现 Agent 接口。
type DataQueryAgent struct {
	Recognizer   intent.Recognizer
	Generator    dsl.Generator
	Validator    dsl.Validator
	Exec         executor.Executor
	MetaStore    metadata.Store
	SessionStore intent.DataSessionStore
	Config       DataQueryConfig
}

// Run 实现 Agent。
func (a *DataQueryAgent) Run(ctx context.Context, req *Request) (*Response, error) {
	if req == nil || len(req.Messages) == 0 {
		return &Response{Text: "请发送一条数据查询或操作请求。"}, nil
	}
	sessionID := getSessionID(req)
	sessCtx, _ := a.SessionStore.Get(sessionID)
	if sessCtx == nil {
		sessCtx = &intent.DataSessionContext{}
		if a.Config.DefaultDatasourceID != "" {
			sessCtx.DatasourceID = a.Config.DefaultDatasourceID
		}
	}
	// 待确认超时：超过约定时间则清除
	if a.Config.ConfirmTimeoutSec > 0 && sessCtx.PendingConfirmDSL != "" {
		if time.Now().Unix()-sessCtx.PendingConfirmAt > a.Config.ConfirmTimeoutSec {
			sessCtx.PendingConfirmDSL = ""
			sessCtx.PendingConfirmDesc = ""
			sessCtx.PendingConfirmAt = 0
			a.SessionStore.Set(sessionID, sessCtx)
		}
	}
	dsID := sessCtx.DatasourceID
	if dsID == "" {
		return &Response{Text: "请先指定数据源（或由管理员配置默认数据源）。"}, nil
	}
	meta, _ := a.MetaStore.GetSchema(ctx, dsID)

	// 本轮是否为“确认执行”请求
	if confirmToken, _ := req.Metadata["confirm_token"].(string); confirmToken != "" && sessCtx.PendingConfirmDSL != "" {
		// 简单策略：有 token 且与待确认 DSL 对应即执行（可改为 token 与 PendingConfirm 绑定校验）
		opts := executor.ExecuteOptions{
			Timeout:  a.Config.Timeout,
			MaxRows:  a.Config.MaxRows,
			ReadOnly: a.Config.ReadOnly,
		}
		res, err := a.Exec.Execute(ctx, dsID, sessCtx.PendingConfirmDSL, opts)
		a.SessionStore.Set(sessionID, &intent.DataSessionContext{
			DatasourceID: dsID,
			LastDSL:     sessCtx.PendingConfirmDSL,
			LastIntent:  sessCtx.LastIntent,
		})
		if err != nil {
			return &Response{Text: "执行失败：" + err.Error()}, nil
		}
		text := formatResult(res)
		return &Response{Text: text}, nil
	}

	lastMsg := req.Messages[len(req.Messages)-1]
	content := lastMsg.Content
	if content == "" {
		return &Response{Text: "未收到有效消息。"}, nil
	}

	parsed, err := a.Recognizer.Recognize(ctx, sessionID, req.Messages, meta)
	if err != nil {
		return &Response{Text: "意图识别失败：" + err.Error()}, nil
	}
	if len(parsed.UncertainFields) > 0 {
		return &Response{Text: "需要澄清：" + fmt.Sprint(parsed.UncertainFields)}, nil
	}

	if parsed.Intent == intent.IntentMetadata {
		text := formatMetadata(meta)
		return &Response{Text: text}, nil
	}

	generatedDSL, params, err := a.Generator.Generate(ctx, parsed, meta)
	if err != nil {
		return &Response{Text: "生成语句失败：" + err.Error()}, nil
	}
	if err := a.Validator.Validate(ctx, generatedDSL, meta, a.Config.ReadOnly); err != nil {
		return &Response{Text: "校验失败：" + err.Error()}, nil
	}

	isWrite := parsed.Intent == intent.IntentInsert || parsed.Intent == intent.IntentUpdate || parsed.Intent == intent.IntentDelete
	if isWrite {
		// 修改类：返回待确认，不直接执行
		confirmReq := dsl.ConfirmRequest{DSL: generatedDSL, Description: parsed.RawNL}
		resp := &Response{
			Text: "请确认是否执行：\n" + generatedDSL + "\n回复「确认」或携带 confirm_token 以执行。",
			Metadata: map[string]any{
				"confirm_required": true,
				"confirm_request":  confirmReq,
			},
		}
		sessCtx.PendingConfirmDSL = generatedDSL
		sessCtx.PendingConfirmDesc = parsed.RawNL
		sessCtx.PendingConfirmAt = time.Now().Unix()
		sessCtx.LastIntent = parsed.Intent
		sessCtx.LastTable = parsed.Entities.Table
		a.SessionStore.Set(sessionID, sessCtx)
		return resp, nil
	}

	opts := executor.ExecuteOptions{
		Timeout:  a.Config.Timeout,
		MaxRows:  a.Config.MaxRows,
		ReadOnly: a.Config.ReadOnly,
		Params:   params,
	}
	res, err := a.Exec.Execute(ctx, dsID, generatedDSL, opts)
	if err != nil {
		if errors.Is(err, errs.ErrBadRequest) {
			return &Response{Text: "请求不合法：" + res.Error}, nil
		}
		return &Response{Text: "执行失败：" + err.Error()}, nil
	}
	text := formatResult(res)
	sessCtx.LastDSL = generatedDSL
	sessCtx.LastTable = parsed.Entities.Table
	sessCtx.LastIntent = parsed.Intent
	a.SessionStore.Set(sessionID, sessCtx)
	return &Response{Text: text, Metadata: map[string]any{"rows": res.Rows, "columns": res.Columns}}, nil
}

func getSessionID(req *Request) string {
	if req == nil || req.Metadata == nil {
		return ""
	}
	if s, ok := req.Metadata["session_id"].(string); ok {
		return s
	}
	return req.RequestID
}

func formatResult(res *executor.Result) string {
	if res == nil {
		return "无结果"
	}
	if res.Error != "" {
		return "错误：" + res.Error
	}
	if len(res.Columns) > 0 && len(res.Rows) > 0 {
		return fmt.Sprintf("共 %d 行。\n列：%v\n数据：%v", len(res.Rows), res.Columns, res.Rows)
	}
	if res.AffectedRows > 0 {
		return "影响行数：" + strconv.Itoa(res.AffectedRows)
	}
	return "执行完成。"
}

func formatMetadata(meta *metadata.Schema) string {
	if meta == nil || len(meta.Tables) == 0 {
		return "当前无表信息。"
	}
	s := "表列表："
	for i, t := range meta.Tables {
		if t != nil {
			if i > 0 {
				s += "、"
			}
			s += t.Name
		}
	}
	return s
}
