package tool

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sath/auth"
	"github.com/sath/events"
	"github.com/sath/executor"
	"github.com/sath/obs"
)

// PendingWrite 表示一次待确认的写/改操作。
type PendingWrite struct {
	Token        string    `json:"token"`
	DSL          string    `json:"dsl"`
	DatasourceID string    `json:"datasource_id"`
	UserID       string    `json:"user_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// WritePendingStore 抽象会话内 pending 写操作的存取。
// 具体实现可以基于会话存储、内存 map 或外部缓存，在 T-08 中完善。
type WritePendingStore interface {
	SavePending(ctx context.Context, sessionID string, p PendingWrite) error
	GetPending(ctx context.Context, sessionID, token string) (*PendingWrite, error)
	DeletePending(ctx context.Context, sessionID, token string) error
}

// TokenGenerator 用于生成写/改确认 token。
type TokenGenerator interface {
	NewToken() (string, error)
}

// RandomTokenGenerator 使用 crypto/rand 生成十六进制随机 token。
type RandomTokenGenerator struct {
	// BytesLen 为随机字节长度，若为 0 则默认 16 字节（32 字符十六进制）。
	BytesLen int
}

func (g RandomTokenGenerator) NewToken() (string, error) {
	n := g.BytesLen
	if n <= 0 {
		n = 16
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ExecuteWriteConfig 构造 execute_write 工具的依赖。
type ExecuteWriteConfig struct {
	Exec                executor.Executor
	Checker             auth.Checker
	PendingStore        WritePendingStore
	TokenGen            TokenGenerator
	DefaultDatasourceID string
	// ConfirmTTLSeconds token 默认有效期（秒），0 时使用默认 300 秒。
	ConfirmTTLSeconds int
	// DefaultTimeoutSec 写操作执行默认超时时间（秒），0 表示不限制。
	DefaultTimeoutSec int
}

// ExecuteWritePendingResponse 表示「提议写/改」阶段的返回结果。
type ExecuteWritePendingResponse struct {
	Status    string `json:"status"`     // "pending"
	Token     string `json:"token"`      // 确认 token
	DSL       string `json:"dsl"`        // 待执行的写/改 DSL
	ExpiresIn int    `json:"expires_in"` // token 预计在多少秒后过期
}

// RegisterExecuteWriteTool 向注册表中注册 execute_write 工具。
// opts 可选：若 opts 中 Description 非空则覆盖默认描述（用于按数据源类型差异化表述）。
func RegisterExecuteWriteTool(r *Registry, cfg *ExecuteWriteConfig, opts ...*RegisterToolOptions) error {
	desc := "Propose and confirm write/change DSL with permission check and confirmation token."
	if len(opts) > 0 && opts[0] != nil && opts[0].Description != "" {
		desc = opts[0].Description
	}
	return r.Register(Tool{
		Name:        "execute_write",
		Description: desc,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dsl": map[string]any{
					"type":        "string",
					"description": "Write/change DSL to execute (e.g. INSERT/UPDATE/DELETE).",
				},
				"confirm_token": map[string]any{
					"type":        "string",
					"description": "Confirmation token returned from a previous proposal call.",
				},
				"datasource_id": map[string]any{
					"type":        "string",
					"description": "Datasource ID; if omitted, the session default is used.",
				},
				"session_id": map[string]any{
					"type":        "string",
					"description": "Session ID used to bind pending writes.",
				},
				"user_id": map[string]any{
					"type":        "string",
					"description": "User ID for permission checks and auditing.",
				},
				"timeout_sec": map[string]any{
					"type":        "integer",
					"description": "Execution timeout in seconds for confirmed writes; non-negative.",
				},
			},
			"required": []string{"dsl"},
		},
		Execute: buildExecuteWriteExecute(cfg),
	})
}

func buildExecuteWriteExecute(cfg *ExecuteWriteConfig) ExecuteFunc {
	return func(ctx context.Context, params map[string]any) (any, error) {
		start := time.Now()
		status := "ok"
		defer func() {
			obs.ObserveDataQueryTool("execute_write", status, time.Since(start))
		}()

		if cfg == nil || cfg.Exec == nil || cfg.PendingStore == nil || cfg.TokenGen == nil {
			status = "error"
			return nil, errors.New("execute_write: not configured (missing executor/store/token generator)")
		}

		datasourceID := cfg.DefaultDatasourceID
		if v := params["datasource_id"]; v != nil {
			if s, ok := v.(string); ok && s != "" {
				datasourceID = s
			}
		}
		if datasourceID == "" {
			status = "error"
			return nil, errors.New("execute_write: datasource_id is required (or set default)")
		}

		rawDSL, _ := params["dsl"]
		dsl, ok := rawDSL.(string)
		if !ok || dsl == "" {
			status = "error"
			return nil, errors.New("execute_write: dsl is required and must be a non-empty string")
		}

		sessionID, _ := params["session_id"].(string)
		userID, _ := params["user_id"].(string)
		if sessionID == "" {
			status = "error"
			return nil, errors.New("execute_write: session_id is required")
		}
		// userID 允许为空，由权限系统自行决定如何处理；建议上层始终传递。

		if token, ok := params["confirm_token"].(string); ok && token != "" {
			// 确认阶段：根据 token 查找 pending 并执行。
			res, err := confirmWrite(ctx, cfg, sessionID, token)
			if err != nil {
				status = "error"
			}
			return res, err
		}

		// 提议阶段：先进行权限检查，再写入 pending。
		res, err := proposeWrite(ctx, cfg, datasourceID, sessionID, userID, dsl)
		if err != nil {
			status = "error"
		}
		return res, err
	}
}

func proposeWrite(ctx context.Context, cfg *ExecuteWriteConfig, datasourceID, sessionID, userID, dsl string) (any, error) {
	if cfg.Checker != nil {
		if ok := cfg.Checker.CanExecute(ctx, userID, datasourceID, dsl); !ok {
			return nil, errors.New("execute_write: permission denied")
		}
	}

	ttl := cfg.ConfirmTTLSeconds
	if ttl <= 0 {
		ttl = 300
	}
	token, err := cfg.TokenGen.NewToken()
	if err != nil {
		return nil, fmt.Errorf("execute_write: generate token: %w", err)
	}
	now := time.Now()
	pw := PendingWrite{
		Token:        token,
		DSL:          dsl,
		DatasourceID: datasourceID,
		UserID:       userID,
		CreatedAt:    now,
	}
	if err := cfg.PendingStore.SavePending(ctx, sessionID, pw); err != nil {
		return nil, fmt.Errorf("execute_write: save pending: %w", err)
	}

	// 审计：写/改提议事件
	if bus := events.DefaultBus(); bus != nil {
		bus.Publish(ctx, events.Event{
			Kind: events.DataQueryWriteProposed,
			Payload: map[string]any{
				"user_id":       userID,
				"session_id":    sessionID,
				"datasource_id": datasourceID,
				"dsl":           dsl,
				"token":         token,
				"status":        "pending",
				"confirmed":     false,
			},
			RequestID: requestIDFromContext(ctx),
		})
	}

	return ExecuteWritePendingResponse{
		Status:    "pending",
		Token:     token,
		DSL:       dsl,
		ExpiresIn: ttl,
	}, nil
}

func confirmWrite(ctx context.Context, cfg *ExecuteWriteConfig, sessionID, token string) (any, error) {
	pw, err := cfg.PendingStore.GetPending(ctx, sessionID, token)
	if err != nil {
		return nil, fmt.Errorf("execute_write: load pending: %w", err)
	}
	if pw == nil {
		return nil, errors.New("execute_write: invalid or expired token")
	}

	ttl := cfg.ConfirmTTLSeconds
	if ttl <= 0 {
		ttl = 300
	}
	if time.Since(pw.CreatedAt) > time.Duration(ttl)*time.Second {
		_ = cfg.PendingStore.DeletePending(ctx, sessionID, token)
		return nil, errors.New("execute_write: token expired")
	}

	// 通过有效期校验后，无论执行成功或失败，该 token 都只能使用一次。
	defer cfg.PendingStore.DeletePending(ctx, sessionID, token)

	timeout := cfg.DefaultTimeoutSec
	if v, ok := ctx.Value("execute_write_timeout_sec").(int); ok && v >= 0 {
		timeout = v
	}

	opts := executor.ExecuteOptions{
		Timeout:  timeout,
		ReadOnly: false,
		Params:   nil,
	}
	res, err := cfg.Exec.Execute(ctx, pw.DatasourceID, pw.DSL, opts)

	success := err == nil
	var execErr any
	if err != nil {
		execErr = err.Error()
	}

	// 审计：写/改实际执行事件（无论成功或失败）
	if bus := events.DefaultBus(); bus != nil {
		bus.Publish(ctx, events.Event{
			Kind: events.DataQueryWriteExecuted,
			Payload: map[string]any{
				"user_id":       pw.UserID,
				"session_id":    sessionID,
				"datasource_id": pw.DatasourceID,
				"dsl":           pw.DSL,
				"token":         token,
				"confirmed":     true,
				"success":       success,
				"error":         execErr,
				"affected_rows": func() int64 {
					if res != nil {
						return res.AffectedRows
					}
					return 0
				}(),
			},
			RequestID: requestIDFromContext(ctx),
		})
	}

	if err != nil {
		return nil, fmt.Errorf("execute_write: %w", err)
	}
	return res, nil
}

// InMemoryWritePendingStore 是基于内存 map 的简单实现，适用于单进程开发与测试环境。
type InMemoryWritePendingStore struct {
	mu    sync.RWMutex
	items map[string]PendingWrite
}

// NewInMemoryWritePendingStore 创建一个新的内存写确认存储。
func NewInMemoryWritePendingStore() *InMemoryWritePendingStore {
	return &InMemoryWritePendingStore{
		items: make(map[string]PendingWrite),
	}
}

func (s *InMemoryWritePendingStore) key(sessionID, token string) string {
	return sessionID + ":" + token
}

func (s *InMemoryWritePendingStore) SavePending(ctx context.Context, sessionID string, p PendingWrite) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[s.key(sessionID, p.Token)] = p
	return nil
}

func (s *InMemoryWritePendingStore) GetPending(ctx context.Context, sessionID, token string) (*PendingWrite, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.items[s.key(sessionID, token)]; ok {
		cp := p
		return &cp, nil
	}
	return nil, nil
}

func (s *InMemoryWritePendingStore) DeletePending(ctx context.Context, sessionID, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, s.key(sessionID, token))
	return nil
}

// requestIDFromContext 尝试从 ctx 中提取 RequestID，用于审计关联。
func requestIDFromContext(ctx context.Context) string {
	if v := ctx.Value("request_id"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
