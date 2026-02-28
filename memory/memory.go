package memory

import (
	"context"

	"github.com/sath/model"
)

// Entry 表示一条记忆记录。
type Entry struct {
	Message model.Message
}

// Memory 定义会话记忆接口，V0.1 仅实现短期 BufferMemory。
type Memory interface {
	Add(ctx context.Context, entry Entry) error
	GetRecent(ctx context.Context, n int) ([]Entry, error)
	Clear(ctx context.Context) error
}
