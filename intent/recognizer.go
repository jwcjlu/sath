package intent

import (
	"context"

	"github.com/sath/metadata"
	"github.com/sath/model"
)

// Recognizer 意图识别器：根据会话消息与可选元数据输出结构化 ParsedInput。
type Recognizer interface {
	Recognize(ctx context.Context, sessionID string, messages []model.Message, meta *metadata.Schema) (*ParsedInput, error)
}
