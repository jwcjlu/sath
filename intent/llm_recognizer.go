package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sath/metadata"
	"github.com/sath/model"
)

// LLMRecognizer 使用 model.Model 做意图与实体抽取，输出 JSON 后解析为 ParsedInput。
type LLMRecognizer struct {
	Model model.Model
}

// Recognize 实现 Recognizer：构造 prompt，调用 Chat，解析 JSON 到 ParsedInput。
func (r *LLMRecognizer) Recognize(ctx context.Context, sessionID string, messages []model.Message, meta *metadata.Schema) (*ParsedInput, error) {
	if r.Model == nil {
		return nil, fmt.Errorf("intent: no model")
	}
	metaHint := ""
	if meta != nil {
		var tables []string
		for _, t := range meta.Tables {
			if t != nil {
				tables = append(tables, t.Name)
			}
		}
		metaHint = fmt.Sprintf("已知表：%s。", strings.Join(tables, ", "))
	}
	sysContent := fmt.Sprintf(`你是数据查询助手。从用户消息中识别意图并抽取实体，仅输出一段 JSON，不要其他说明。
意图取值：query/insert/update/delete/metadata/rewrite。
实体包含：datasource_id、schema、table、columns、conditions、aggregations、order_by、limit、offset、set_clause、values 等。
%s
输出格式示例：{"intent":"query","entities":{"table":"users","columns":["id","name"],"limit":10},"raw_nl":"用户原文"}`,
		metaHint)
	chatMsgs := make([]model.Message, 0, len(messages)+1)
	chatMsgs = append(chatMsgs, model.Message{Role: "system", Content: sysContent})
	chatMsgs = append(chatMsgs, messages...)
	gen, err := r.Model.Chat(ctx, chatMsgs)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(gen.Text)
	if text == "" {
		return nil, fmt.Errorf("intent: empty model response")
	}
	jsonBytes := extractJSON(text)
	var out ParsedInput
	if err := json.Unmarshal(jsonBytes, &out); err != nil {
		return nil, fmt.Errorf("intent: parse json: %w", err)
	}
	return &out, nil
}

// extractJSON 从文本中取第一段完整 JSON（去 markdown 代码块等）。
func extractJSON(s string) []byte {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		s = strings.TrimLeft(s, " \t\n")
		if strings.HasPrefix(s, "json") {
			if nl := strings.Index(s, "\n"); nl >= 0 {
				s = s[nl+1:]
			}
		}
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return []byte(s)
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return []byte(s[start : i+1])
			}
		}
	}
	return []byte(s[start:])
}
