package parser

import (
	"bytes"
	"encoding/json"
)

// JSONParser 使用 encoding/json 将输出解析为结构化数据。
// 支持两种模式：
// - 宽松模式：尝试从包含 JSON 的文本中提取第一段完整 JSON。
// - 严格模式：要求整个文本是合法 JSON。
type JSONParser struct {
	Strict bool
}

func NewJSONParser(strict bool) *JSONParser {
	return &JSONParser{Strict: strict}
}

func (p *JSONParser) Parse(text string, v any) error {
	if p.Strict {
		return json.Unmarshal([]byte(text), v)
	}
	// 宽松模式：尝试在文本中找到第一段 JSON（从第一个 '{' 或 '[' 开始）。
	start := -1
	for i, r := range text {
		if r == '{' || r == '[' {
			start = i
			break
		}
	}
	if start == -1 {
		// 退化为严格模式，返回错误信息。
		return json.Unmarshal([]byte(text), v)
	}
	segment := text[start:]
	decoder := json.NewDecoder(bytes.NewBufferString(segment))
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
