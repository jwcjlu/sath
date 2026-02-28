package parser

// Parser 定义输出解析器的通用接口。
// 典型用法：将 LLM 的字符串输出转换为结构化数据（struct/map 等）。
type Parser interface {
	Parse(text string, v any) error
}
