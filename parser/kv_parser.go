package parser

import (
	"bufio"
	"strings"
)

// KVParser 解析形如 "key: value" 的行文本为 map[string]string。
// 忽略空行和不含分隔符的行。
type KVParser struct {
	Separator string // 默认 ":"
}

func NewKVParser() *KVParser {
	return &KVParser{Separator: ":"}
}

func (p *KVParser) Parse(text string, v any) error {
	m, ok := v.(*map[string]string)
	if !ok {
		return &TypeError{Expected: "*map[string]string"}
	}
	if p.Separator == "" {
		p.Separator = ":"
	}
	res := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		idx := strings.Index(line, p.Separator)
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+len(p.Separator):])
		if key == "" {
			continue
		}
		res[key] = value
	}
	*m = res
	return nil
}

// TypeError 在调用方传入不兼容目标类型时返回。
type TypeError struct {
	Expected string
}

func (e *TypeError) Error() string {
	return "parser: expected " + e.Expected
}
