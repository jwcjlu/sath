package parser

import (
	"errors"
	"strings"
)

// TableParser 将 Markdown 或简单文本表格解析为 [][]string。
// 支持 | a | b | 与制表符/多空格分隔的简单表格。
type TableParser struct {
	// TrimCells 是否去除每格首尾空白，默认 true。
	TrimCells bool
}

// NewTableParser 返回默认的表格解析器。
func NewTableParser() *TableParser {
	return &TableParser{TrimCells: true}
}

// Parse 将 text 解析为表格并写入 v。v 须为 *[][]string 或 *[]map[string]string（首行为键）。
// 若 v 为 *[][]string，则每行对应一行切片；若为 *[]map[string]string，则首行作为 key，后续每行转为 map。
func (p *TableParser) Parse(text string, v any) error {
	rows := p.parseRows(text)
	if len(rows) == 0 {
		return nil
	}
	switch dst := v.(type) {
	case *[][]string:
		*dst = rows
		return nil
	case *[]map[string]string:
		keys := rows[0]
		out := make([]map[string]string, 0, len(rows)-1)
		for i := 1; i < len(rows); i++ {
			m := make(map[string]string)
			for j, k := range keys {
				if j < len(rows[i]) {
					m[k] = rows[i][j]
				}
			}
			out = append(out, m)
		}
		*dst = out
		return nil
	default:
		return errUnsupportedTableDest
	}
}

var errUnsupportedTableDest = errors.New("table parser: v must be *[][]string or *[]map[string]string")

func (p *TableParser) parseRows(text string) [][]string {
	var rows [][]string
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var cells []string
		if strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			for _, s := range parts {
				s = strings.TrimSpace(s)
				if p.TrimCells {
					s = strings.TrimSpace(s)
				}
				cells = append(cells, s)
			}
			if len(cells) > 0 && cells[0] == "" {
				cells = cells[1:]
			}
			if len(cells) > 0 && cells[len(cells)-1] == "" {
				cells = cells[:len(cells)-1]
			}
		} else {
			for _, s := range strings.Fields(line) {
				cells = append(cells, s)
			}
		}
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}
	return rows
}
