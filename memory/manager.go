package memory

import (
	"context"
	"fmt"

	"github.com/sath/model"
)

// Manager 将短期记忆、向量记忆与摘要记忆协调在一起。
// - 短期：最近 N 条对话（BufferMemory）
// - 长期：向量检索（VectorStore）
// - 摘要：对过长历史做压缩（SummaryMemory）
type Manager struct {
	short    *BufferMemory
	vector   VectorStore
	summary  *SummaryMemory
	model    model.Model
	maxShort int
}

type ManagerConfig struct {
	MaxShortHistory int
}

func NewManager(m model.Model, vectorStore VectorStore, summary *SummaryMemory, cfg ManagerConfig) *Manager {
	if cfg.MaxShortHistory <= 0 {
		cfg.MaxShortHistory = 20
	}
	return &Manager{
		short:    NewBufferMemory(cfg.MaxShortHistory),
		vector:   vectorStore,
		summary:  summary,
		model:    m,
		maxShort: cfg.MaxShortHistory,
	}
}

// AddMessage 将一条最新对话写入短期记忆，必要时触发摘要与向量写入。
// id 用于在向量记忆和摘要记忆中标识该条记录来源（例如会话 ID + 序号）。
func (m *Manager) AddMessage(ctx context.Context, id string, msg model.Message, meta map[string]any) error {
	if err := m.short.Add(ctx, Entry{Message: msg}); err != nil {
		return err
	}

	// 对用户和助手的消息写入向量记忆，便于长期检索。
	if m.vector != nil && (msg.Role == "user" || msg.Role == "assistant") {
		if err := EmbedAndAdd(ctx, m.model, m.vector, id, msg.Content, meta); err != nil {
			return fmt.Errorf("vector embed and add: %w", err)
		}
	}

	// 当短期记忆接近上限时，对当前短期历史做一次摘要，写入摘要记忆。
	if m.summary != nil {
		history, _ := m.short.GetRecent(ctx, m.maxShort)
		if len(history) >= m.maxShort {
			msgs := make([]model.Message, 0, len(history))
			for _, e := range history {
				msgs = append(msgs, e.Message)
			}
			if _, err := m.summary.SummarizeAndAdd(ctx, id, msgs); err != nil {
				return fmt.Errorf("summary add: %w", err)
			}
			// 清空短期记忆，留给后续对话。
			_ = m.short.Clear(ctx)
		}
	}
	return nil
}

// ShortRecent 返回最近 N 条短期记忆。
func (m *Manager) ShortRecent(ctx context.Context, n int) ([]Entry, error) {
	return m.short.GetRecent(ctx, n)
}

// SearchLong 使用向量记忆在长期记忆中进行检索。
func (m *Manager) SearchLong(ctx context.Context, queryText string, k int) ([]VectorEntry, error) {
	if m.vector == nil {
		return nil, nil
	}
	embs, err := m.model.Embed(ctx, []string{queryText})
	if err != nil {
		return nil, err
	}
	if len(embs) == 0 {
		return nil, nil
	}
	return m.vector.Search(ctx, embs[0].Vector, k)
}

// Summaries 返回当前所有摘要记忆（按时间顺序）。
func (m *Manager) Summaries() []SummaryEntry {
	if m.summary == nil {
		return nil
	}
	return m.summary.List()
}
