package memory

import (
	"context"
	"strings"

	"github.com/sath/model"
)

// SummaryEntry 表示一条摘要记忆。
type SummaryEntry struct {
	ID      string
	Summary string
}

// SummaryMemory 负责将长对话压缩为较短的摘要文本。
// 这里实现一个最小可用版本：在触发条件满足时，将最近若干条对话拼接后，
// 调用模型的 Generate 得到摘要，并存入内部切片。
type SummaryMemory struct {
	model    model.Model
	entries  []SummaryEntry
	maxItems int
}

func NewSummaryMemory(m model.Model, maxItems int) *SummaryMemory {
	if maxItems <= 0 {
		maxItems = 100
	}
	return &SummaryMemory{
		model:    m,
		entries:  make([]SummaryEntry, 0, maxItems),
		maxItems: maxItems,
	}
}

// SummarizeAndAdd 将一批对话消息汇总为摘要并存入摘要记忆。
func (s *SummaryMemory) SummarizeAndAdd(ctx context.Context, id string, messages []model.Message) (SummaryEntry, error) {
	var b strings.Builder
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	prompt := "请对以下对话内容做一个简洁的摘要，保留关键信息和结论：\n\n" + b.String()
	gen, err := s.model.Generate(ctx, prompt)
	if err != nil {
		return SummaryEntry{}, err
	}

	entry := SummaryEntry{
		ID:      id,
		Summary: gen.Text,
	}
	if len(s.entries) >= s.maxItems {
		copy(s.entries, s.entries[1:])
		s.entries[len(s.entries)-1] = entry
	} else {
		s.entries = append(s.entries, entry)
	}
	return entry, nil
}

func (s *SummaryMemory) List() []SummaryEntry {
	out := make([]SummaryEntry, len(s.entries))
	copy(out, s.entries)
	return out
}
