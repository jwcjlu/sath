package templates

import (
	"context"
	"fmt"
	"strings"

	"github.com/sath/agent"
	"github.com/sath/auth"
	"github.com/sath/config"
	"github.com/sath/datasource"
	"github.com/sath/executor"
	"github.com/sath/memory"
	"github.com/sath/metadata"
	"github.com/sath/middleware"
	"github.com/sath/model"
	"github.com/sath/tool"
)

// DataQueryPromptConfig 配置数据查询 Agent 的系统提示。
type DataQueryPromptConfig struct {
	// DatasourceType 如 "mysql"、"postgres"。仅用于文案描述。
	DatasourceType string
	// AllowWrite 为 false 时仅允许只读工具（不应调用 execute_write）。
	AllowWrite bool
}

// BuildDataQuerySystemPrompt 构造数据查询 Agent 的系统级提示，指导模型按 ReAct + 工具调用方式工作。
// 该提示应作为首条 system message 注入对话。
func BuildDataQuerySystemPrompt(cfg DataQueryPromptConfig) string {
	ds := cfg.DatasourceType
	if ds == "" {
		ds = "SQL"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "你是一个安全可靠的数据查询助手，负责通过工具访问 %s 数据源，", ds)
	b.WriteString("帮助用户用自然语言完成数据的「列举、结构查看、只读查询，以及在允许时的写/改」任务。\n\n")

	b.WriteString("【总体原则】\n")
	b.WriteString("1. 严格遵守只读/写改边界：除非明确允许写/改且用户有强烈意图，否则不要修改数据；永远不要直接编造查询结果。\n")
	if !cfg.AllowWrite {
		b.WriteString("2. 当前运行在「只读模式」，你**禁止调用**任何会修改数据的工具（如 execute_write）。若用户请求写/改，请解释当前仅支持查询，并给出只读替代方案。\n")
	} else {
		b.WriteString("2. 写/改操作必须经过**两阶段确认**：先提议（生成确认 token），等待用户基于该 token 进行自然语言确认，只有在用户明确同意且携带 token 时才真正执行。\n")
	}
	b.WriteString("3. 始终用简洁的中文回答用户，必要时在结果后面补充简短的业务含义解释。\n\n")

	b.WriteString("【可用工具与用途】\n")
	b.WriteString("- list_tables：列举当前数据源中的所有表/集合及简要说明，用于熟悉数据对象。\n")
	b.WriteString("- describe_table：查看某张表的列、类型、含义，用于在编写查询前理解结构。\n")
	b.WriteString("- execute_read：执行只读查询（通常是 SELECT），返回表格结果，适合统计、明细查询等。\n")
	if cfg.AllowWrite {
		b.WriteString("- execute_write：用于 INSERT/UPDATE/DELETE 等写/改操作，只能在用户已经明确确认且你持有有效 confirm_token 时使用。\n")
	} else {
		b.WriteString("- （禁用）execute_write：当前环境为只读模式，你不应调用该工具。\n")
	}
	b.WriteString("\n")

	b.WriteString("【推荐工作流（ReAct 思考→行动→观察）】\n")
	b.WriteString("1. 探索阶段：当用户第一次接入或你不了解库结构时，先使用 list_tables 获取有哪些业务表，再根据需要对关键表使用 describe_table。\n")
	b.WriteString("2. 计划阶段：根据用户问题和表结构，在「思考」中用自然语言说明你打算查询哪些表、使用哪些条件与聚合字段。\n")
	b.WriteString("3. 只读查询阶段：\n")
	b.WriteString("   - 使用 execute_read 执行 SELECT 语句；优先返回汇总后的结果（如总数、汇总金额），必要时再补充明细。\n")
	b.WriteString("   - 查询前应简要描述你即将执行的查询意图（不需要向用户展示 SQL，仅在思考中自我说明）。\n")
	if cfg.AllowWrite {
		b.WriteString("4. 写/改提议阶段（仅在用户明确要求修改数据时）：\n")
		b.WriteString("   - 首先在「思考」中确认用户的真实意图、影响范围和风险；如有歧义，先向用户再确认。\n")
		b.WriteString("   - 使用 execute_write，仅传入 dsl（不带 confirm_token）提出写/改建议，获取包含 token 的待确认信息。\n")
		b.WriteString("   - 将 token 和拟进行的变更以自然语言总结给用户，提醒其确认风险。\n")
		b.WriteString("5. 写/改确认与执行阶段：\n")
		b.WriteString("   - 只有当用户明确基于该 token 表示同意执行时，才再次调用 execute_write，携带 confirm_token 完成真正执行。\n")
		b.WriteString("   - 执行完成后，向用户说明执行结果（受影响行数等），并强调变更已经落库，必要时给出回滚建议或检查方式。\n")
	}
	b.WriteString("6. 解读阶段：在拿到查询或写/改结果后，用业务友好的语言总结关键结论，而不是简单重复原始表格。\n\n")

	b.WriteString("【回答格式要求】\n")
	b.WriteString("1. 对于复杂问题，你可以在内部进行多步思考和多轮工具调用，但对用户的可见回答应当是一个**自然语言总结**。\n")
	b.WriteString("2. 当你认为需要使用工具时，直接调用相应工具（由系统负责工具 API 的封装），不需要在回答中描述调用细节。\n")
	if cfg.AllowWrite {
		b.WriteString("3. 当用户请求写/改但尚未给出最终确认时，请明确告知：当前只完成了「变更方案的提议」，尚未真正执行，需用户基于 token 确认。\n")
	} else {
		b.WriteString("3. 当用户请求写/改时，请说明当前仅支持只读查询，并可给出如何通过查询来验证或评估的建议。\n")
	}
	b.WriteString("4. 若查询结果为空或不确定，请直接说明，并给出可能的原因或下一步排查建议。\n\n")

	b.WriteString("【示例场景】\n")
	b.WriteString("1. 列举表：\n")
	b.WriteString("   - 用户：\"这个库里有哪些和订单相关的表？\"\n")
	b.WriteString("   - 你：调用 list_tables，筛选名称或注释中包含「order」的表，然后用自然语言总结。\n")
	b.WriteString("2. 查看结构：\n")
	b.WriteString("   - 用户：\"帮我看看订单表的字段。\"\n")
	b.WriteString("   - 你：调用 describe_table（如 table_name=\"orders\"），并解释关键字段（如订单状态、金额、时间）。\n")
	b.WriteString("3. 只读查询：\n")
	b.WriteString("   - 用户：\"统计本月每个渠道的订单数和金额。\"\n")
	b.WriteString("   - 你：在思考中说明将使用 orders 表、按渠道与月份分组统计，然后通过 execute_read 执行 SELECT，最后用中文总结结果。\n")
	if cfg.AllowWrite {
		b.WriteString("4. 写/改带确认：\n")
		b.WriteString("   - 用户：\"把昨天所有测试环境的订单状态改为已取消。\"\n")
		b.WriteString("   - 你：\n")
		b.WriteString("     a) 先通过只读查询（execute_read）确认影响范围与数量；\n")
		b.WriteString("     b) 再使用 execute_write 提议一个 UPDATE 语句，获取确认 token；\n")
		b.WriteString("     c) 告知用户「将修改多少条记录」以及 token，让用户确认；\n")
		b.WriteString("     d) 只有在用户明确使用该 token 确认后，才调用 execute_write 携带 confirm_token 真正执行。\n")
	}

	return b.String()
}

// DataQueryConfig 描述 DataQueryReActAgent 所需的依赖与行为配置。
type DataQueryConfig struct {
	// DatasourceRegistry 管理已注册的数据源（必须提供）。
	DatasourceRegistry *datasource.Registry
	// MetadataStore 提供 Schema/Table 等元数据缓存（必须提供）。
	MetadataStore *metadata.InMemoryStore
	// Exec 用于执行生成的 DSL（必须提供）。
	Exec executor.Executor
	// Checker 在写/改前进行权限预检，可为 nil 表示不做权限判断。
	Checker auth.Checker

	// 默认数据源 ID（可选，优先级低于请求 Metadata 中的 datasource_id）。
	DefaultDatasourceID string
	// 是否允许写/改；为 false 时不会注册 execute_write 工具。
	AllowWrite bool

	// ReAct 最大步骤数，<=0 时默认为 4。
	MaxReActSteps int

	// 只读查询默认超时与最大行数（可选，0 表示不限制）。
	DefaultReadTimeoutSec int
	DefaultReadMaxRows    int

	// 写/改确认 token 的有效期（秒），<=0 时默认为 300。
	WriteConfirmTTLSeconds int
	// 写/改执行默认超时（秒），0 表示不限制。
	DefaultWriteTimeoutSec int

	// 可选自定义实现；为空时使用内存实现与随机 token。
	PendingStore tool.WritePendingStore
	TokenGen     tool.TokenGenerator
}

// NewDataQueryHandlerFromConfig 根据 Config 装配数据查询 ReAct Agent 与中间件链。
// 若未配置任何 DataSources，将返回错误。
func NewDataQueryHandlerFromConfig(cfg config.Config, middlewareByName map[string]middleware.Middleware) (middleware.Handler, error) {
	if len(cfg.DataSources) == 0 {
		return nil, fmt.Errorf("dataquery: no data_sources configured")
	}

	m, err := model.NewFromIdentifier(cfg.ModelName)
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	mem := memory.NewBufferMemory(cfg.MaxHistory)
	mws := make([]middleware.Middleware, 0, len(cfg.Middlewares)+2)
	mws = append(mws, middleware.RecoveryMiddleware, middleware.LoggingMiddleware)
	for _, name := range cfg.Middlewares {
		if mw, ok := middlewareByName[name]; ok {
			mws = append(mws, mw)
		}
	}

	// 构建数据源 Registry（当前只支持 mysql 类型）。
	dsReg := datasource.NewRegistry()
	datasource.RegisterMySQL(dsReg)
	for _, ds := range cfg.DataSources {
		if ds.Type == "" {
			ds.Type = "mysql"
		}
		if _, err := dsReg.Register(ds); err != nil {
			return nil, fmt.Errorf("dataquery: register datasource %s: %w", ds.ID, err)
		}
	}

	store := metadata.NewInMemoryStore(nil)
	exec := executor.NewMySQLExecutor(dsReg)

	dqCfg := DataQueryConfig{
		DatasourceRegistry:     dsReg,
		MetadataStore:          store,
		Exec:                   exec,
		Checker:                auth.PermissiveChecker{},
		DefaultDatasourceID:    cfg.DefaultDatasourceID,
		AllowWrite:             cfg.DataAllowWrite,
		MaxReActSteps:          4,
		DefaultReadTimeoutSec:  0,
		DefaultReadMaxRows:     0,
		WriteConfirmTTLSeconds: 300,
		DefaultWriteTimeoutSec: 0,
	}

	return NewDataQueryHandler(m, mem, dqCfg, mws...), nil
}

// NewDataQueryHandler 构建一个专用于数据查询的 ReAct Agent 处理器。
// 它会注册 list_tables / describe_table / execute_read / （可选）execute_write 工具，
// 并基于 DataQueryPromptConfig 注入系统提示与会话上下文（session_id、user_id、datasource_id）。
func NewDataQueryHandler(m model.Model, mem memory.Memory, cfg DataQueryConfig, mws ...middleware.Middleware) middleware.Handler {
	if cfg.DatasourceRegistry == nil || cfg.MetadataStore == nil || cfg.Exec == nil {
		// 保持简单的 panic，以便在服务启动阶段就能发现配置问题。
		panic("NewDataQueryHandler: DatasourceRegistry, MetadataStore and Exec are required")
	}

	if cfg.MaxReActSteps <= 0 {
		cfg.MaxReActSteps = 4
	}
	if cfg.WriteConfirmTTLSeconds <= 0 {
		cfg.WriteConfirmTTLSeconds = 300
	}

	// 准备工具注册表。
	reg := tool.NewRegistry()

	// list_tables
	_ = tool.RegisterListTablesTool(reg, &tool.ListTablesConfig{
		Store:               cfg.MetadataStore,
		Registry:            cfg.DatasourceRegistry,
		DefaultDatasourceID: cfg.DefaultDatasourceID,
	})

	// describe_table
	_ = tool.RegisterDescribeTableTool(reg, &tool.DescribeTableConfig{
		Store:               cfg.MetadataStore,
		Registry:            cfg.DatasourceRegistry,
		DefaultDatasourceID: cfg.DefaultDatasourceID,
	})

	// execute_read
	_ = tool.RegisterExecuteReadTool(reg, &tool.ExecuteReadConfig{
		Exec:                cfg.Exec,
		DefaultDatasourceID: cfg.DefaultDatasourceID,
		DefaultTimeoutSec:   cfg.DefaultReadTimeoutSec,
		DefaultMaxRows:      cfg.DefaultReadMaxRows,
	})

	// execute_write（仅在允许写/改时注册）
	if cfg.AllowWrite {
		pending := cfg.PendingStore
		if pending == nil {
			pending = tool.NewInMemoryWritePendingStore()
		}
		tokenGen := cfg.TokenGen
		if tokenGen == nil {
			tokenGen = tool.RandomTokenGenerator{}
		}
		_ = tool.RegisterExecuteWriteTool(reg, &tool.ExecuteWriteConfig{
			Exec:                cfg.Exec,
			Checker:             cfg.Checker,
			PendingStore:        pending,
			TokenGen:            tokenGen,
			DefaultDatasourceID: cfg.DefaultDatasourceID,
			ConfirmTTLSeconds:   cfg.WriteConfirmTTLSeconds,
			DefaultTimeoutSec:   cfg.DefaultWriteTimeoutSec,
		})
	}

	react := agent.NewReActAgent(m, mem, reg, agent.WithReActMaxSteps(cfg.MaxReActSteps))

	final := func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
		if req == nil {
			return nil, nil
		}

		// 从 Metadata 中提取会话/用户/数据源信息。
		var sessionID, userID, datasourceID string
		if req.Metadata != nil {
			if v, ok := req.Metadata["session_id"].(string); ok {
				sessionID = v
			}
			if v, ok := req.Metadata["user_id"].(string); ok {
				userID = v
			}
			if v, ok := req.Metadata["datasource_id"].(string); ok && v != "" {
				datasourceID = v
			}
		}
		if datasourceID == "" {
			datasourceID = cfg.DefaultDatasourceID
		}

		// 构造系统提示，并附加当前上下文信息，作为第一条 system message。
		promptCfg := DataQueryPromptConfig{
			DatasourceType: "mysql", // 当前 MVP 聚焦 MySQL，可按需扩展
			AllowWrite:     cfg.AllowWrite,
		}
		sys := BuildDataQuerySystemPrompt(promptCfg)
		var extra []string
		if datasourceID != "" {
			extra = append(extra, fmt.Sprintf("当前会话默认数据源 ID 为：%s。", datasourceID))
		}
		if sessionID != "" {
			extra = append(extra, fmt.Sprintf("当前会话 session_id 为：%s。", sessionID))
		}
		if userID != "" {
			extra = append(extra, fmt.Sprintf("当前用户 ID 为：%s。", userID))
		}
		if len(extra) > 0 {
			sys = sys + "\n\n" + strings.Join(extra, "\n")
		}

		// 将系统提示注入 messages 之前。
		req2 := *req
		req2.Messages = append([]model.Message{
			{Role: "system", Content: sys},
		}, req.Messages...)

		// 标记 agent_name 以便 metrics 中区分 dataquery 请求。
		if req2.Metadata == nil {
			req2.Metadata = make(map[string]any)
		}
		if _, ok := req2.Metadata["agent_name"]; !ok {
			req2.Metadata["agent_name"] = "dataquery"
		}

		// 将 RequestID 注入 ctx，便于下游工具（如 execute_write）写审计事件时使用。
		if req2.RequestID != "" {
			ctx = context.WithValue(ctx, "request_id", req2.RequestID)
		}

		return react.Run(ctx, &req2)
	}

	return middleware.Chain(final, mws...)
}
