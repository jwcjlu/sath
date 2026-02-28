package cli

import (
	"time"

	"github.com/sath/agent"
	"github.com/sath/config"
	"github.com/sath/datasource"
	"github.com/sath/dsl"
	"github.com/sath/executor"
	"github.com/sath/intent"
	"github.com/sath/metadata"
	"github.com/sath/middleware"
	"github.com/sath/model"
	"github.com/sath/templates"
)

// buildDataQueryHandler 根据配置构建数据对话处理器；无可用 model 时返回 nil。
func buildDataQueryHandler(cfg config.Config, debug bool) middleware.Handler {
	m, err := model.NewFromIdentifier(cfg.ModelName)
	if err != nil {
		m, _ = model.NewOpenAIClient()
	}
	if m == nil {
		return nil
	}
	registry := datasource.NewRegistry()
	datasource.RegisterMySQL(registry)
	metaStore := metadata.NewInMemoryStore()
	sessionStore := intent.NewInMemoryDataSessionStore()
	dataAgent := &agent.DataQueryAgent{
		Recognizer:   &intent.LLMRecognizer{Model: m},
		Generator:    &dsl.MySQLGenerator{},
		Validator:    &dsl.MySQLValidator{},
		Exec:         &executor.MySQLExecutor{Registry: registry},
		MetaStore:    metaStore,
		SessionStore: sessionStore,
		Config: agent.DataQueryConfig{
			Timeout:           30 * time.Second,
			MaxRows:           1000,
			ReadOnly:          false,
			ConfirmTimeoutSec: 300,
		},
	}
	mws := []middleware.Middleware{
		middleware.RecoveryMiddleware,
		middleware.LoggingMiddleware,
	}
	if debug {
		mws = append(mws, middleware.DebugMiddleware(true))
	}
	return templates.NewDataQueryHandler(dataAgent, mws...)
}
