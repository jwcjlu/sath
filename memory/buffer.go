package memory

import "context"

// BufferMemory 使用内存切片存储最近 N 条消息，适合作为会话级短期记忆。
type BufferMemory struct {
	max int
	buf []Entry
}

// NewBufferMemory 创建一个带上限的 BufferMemory。
func NewBufferMemory(max int) *BufferMemory {
	if max <= 0 {
		max = 10
	}
	return &BufferMemory{
		max: max,
		buf: make([]Entry, 0, max),
	}
}

func (m *BufferMemory) Add(_ context.Context, entry Entry) error {
	if len(m.buf) >= m.max {
		copy(m.buf, m.buf[1:])
		m.buf[len(m.buf)-1] = entry
	} else {
		m.buf = append(m.buf, entry)
	}
	return nil
}

func (m *BufferMemory) GetRecent(_ context.Context, n int) ([]Entry, error) {
	if n <= 0 || n >= len(m.buf) {
		out := make([]Entry, len(m.buf))
		copy(out, m.buf)
		return out, nil
	}
	out := make([]Entry, n)
	copy(out, m.buf[len(m.buf)-n:])
	return out, nil
}

func (m *BufferMemory) Clear(_ context.Context) error {
	m.buf = m.buf[:0]
	return nil
}
