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

// 数据查询工具名，与 tool 包注册名一致。
const (
	ToolListTables    = "list_tables"
	ToolDescribeTable = "describe_table"
	ToolExecuteRead   = "execute_read"
	ToolExecuteWrite  = "execute_write"
)

// DefaultToolCapabilitiesByType 按数据源类型声明支持的工具列表；未注册描述符时回退用。
var DefaultToolCapabilitiesByType = map[string][]string{
	"mysql":         {ToolListTables, ToolDescribeTable, ToolExecuteRead, ToolExecuteWrite},
	"hive":          {ToolListTables, ToolDescribeTable, ToolExecuteRead},
	"elasticsearch": {ToolListTables, ToolDescribeTable, ToolExecuteRead},
	"mongodb":       {ToolListTables, ToolDescribeTable, ToolExecuteRead},
}

// ToolDef 描述符中单工具定义：Name 与 tool 包注册名一致，Description 为空则使用 tool 包默认。
type ToolDef struct {
	Name        string
	Description string
}

// TypeDescriptor 数据源类型描述符：工具能力与类型相关提示片段，单一事实来源。
type TypeDescriptor struct {
	Type   string
	Tools  []ToolDef
	Prompt struct {
		Intro         string // 类型相关首段
		ToolSummaries string // 可用工具与用途（仅只读三件套 + 占位；execute_write 由公共逻辑按 AllowWrite 追加）
		Workflow      string // 推荐工作流（探索→计划→只读→解读；写/改由公共逻辑追加）
		Examples      string // 示例场景（列举/结构/只读；写改由公共逻辑追加）
	}
}

var typeDescriptors = make(map[string]*TypeDescriptor)

func init() {
	registerTypeDescriptor(mysqlDescriptor())
	registerTypeDescriptor(hiveDescriptor())
	registerTypeDescriptor(elasticsearchDescriptor())
	registerTypeDescriptor(mongodbDescriptor())
}

func registerTypeDescriptor(d *TypeDescriptor) {
	if d != nil && d.Type != "" {
		typeDescriptors[d.Type] = d
	}
}

// GetDescriptor 返回数据源类型对应的描述符，未注册时返回默认（mysql）。
func GetDescriptor(datasourceType string) *TypeDescriptor {
	if datasourceType == "es" {
		datasourceType = "elasticsearch"
	}
	if d, ok := typeDescriptors[datasourceType]; ok {
		return d
	}
	return typeDescriptors["mysql"]
}

func mysqlDescriptor() *TypeDescriptor {
	return &TypeDescriptor{
		Type: "mysql",
		Tools: []ToolDef{
			{Name: ToolListTables, Description: "List tables (or collections) in the current datasource. Returns table names and optional comments."},
			{Name: ToolDescribeTable, Description: "Describe table structure in the current datasource. Returns columns with type and nullability."},
			{Name: ToolExecuteRead, Description: "Execute a read-only DSL (e.g. SQL SELECT) on the current datasource and return rows."},
			{Name: ToolExecuteWrite, Description: "Propose and confirm write/change DSL with permission check and confirmation token."},
		},
		Prompt: struct {
			Intro         string
			ToolSummaries string
			Workflow      string
			Examples      string
		}{
			Intro: "你是一个安全可靠的数据查询助手，负责通过工具访问 mysql 数据源，帮助用户用自然语言完成数据的「列举、结构查看、只读查询，以及在允许时的写/改」任务。\n\n",
			ToolSummaries: "- list_tables：列举当前数据源中的表/集合及简要说明；可选传 keyword 参数按关键词模糊过滤表名。\n" +
				"- describe_table：查看某张表的列、类型、含义，用于在编写查询前理解结构。\n" +
				"- execute_read：执行只读查询（通常是 SELECT），返回表格结果，适合统计、明细查询等。\n",
			Workflow: "1. 探索阶段：当用户第一次接入或你不了解库结构时，先使用 list_tables 获取有哪些业务表，再根据需要对关键表使用 describe_table。\n" +
				"2. 计划阶段：根据用户问题和表结构，在「思考」中用自然语言说明你打算查询哪些表、使用哪些条件与聚合字段。\n" +
				"3. 只读查询阶段：\n" +
				"   - 使用 execute_read 执行 SELECT 语句；优先返回汇总后的结果（如总数、汇总金额），必要时再补充明细。\n" +
				"   - 查询前应简要描述你即将执行的查询意图（不需要向用户展示 SQL，仅在思考中自我说明）。\n",
			Examples: "1. 列举表：\n" +
				"   - 用户：\"这个库里有哪些和订单相关的表？\"\n" +
				"   - 你：调用 list_tables，筛选名称或注释中包含「order」的表，然后用自然语言总结。\n" +
				"2. 查看结构：\n" +
				"   - 用户：\"帮我看看订单表的字段。\"\n" +
				"   - 你：调用 describe_table（如 table_name=\"orders\"），并解释关键字段（如订单状态、金额、时间）。\n" +
				"3. 只读查询：\n" +
				"   - 用户：\"统计本月每个渠道的订单数和金额。\"\n" +
				"   - 你：在思考中说明将使用 orders 表、按渠道与月份分组统计，然后通过 execute_read 执行 SELECT，最后用中文总结结果。\n",
		},
	}
}

func hiveDescriptor() *TypeDescriptor {
	return &TypeDescriptor{
		Type: "hive",
		Tools: []ToolDef{
			{Name: ToolListTables, Description: "List tables in the current Hive database. Returns table names and optional comments."},
			{Name: ToolDescribeTable, Description: "Describe Hive table structure in the current datasource. Returns columns with type and nullability/partition info if available."},
			{Name: ToolExecuteRead, Description: "Execute a read-only Hive SQL (typically SELECT) on the current datasource and return rows."},
		},
		Prompt: struct {
			Intro         string
			ToolSummaries string
			Workflow      string
			Examples      string
		}{
			Intro: "你是一个安全可靠的数据查询助手，负责通过工具访问 Hive 数据源，帮助用户用自然语言完成数据仓库场景下的「列举、结构查看、只读查询」任务（写/改能力需单独开放）。\n\n",
			ToolSummaries: "- list_tables：列举当前 Hive 数据库中的表（包含明细表、宽表等），可选 keyword 模糊过滤表名。\n" +
				"- describe_table：查看指定 Hive 表的列名、类型，以及（在可用时）分区字段等信息，用于在编写查询前理解数据结构。\n" +
				"- execute_read：执行只读 Hive SQL（通常是 SELECT），支持 WHERE 过滤、GROUP BY、ORDER BY、LIMIT 等，返回表格结果。\n",
			Workflow: "1. 探索阶段：当你不了解当前 Hive 库结构时，先使用 list_tables 获取有哪些表，再使用 describe_table 查看关键表（特别是分区字段与时间字段）。\n" +
				"2. 计划阶段：根据用户问题和表结构，在「思考」中说明你打算使用哪些分区（如按 dt、ds、date 字段限定范围）、哪些维度与指标，以及是否需要聚合。\n" +
				"3. 只读查询阶段：\n" +
				"   - 使用 execute_read 执行 SELECT 语句，**优先加上合理的分区过滤与 LIMIT**，避免全表扫描；可以先做聚合查询，再在必要时补充少量明细。\n" +
				"   - 查询前应简要描述你即将执行的查询意图（不需要向用户展示 SQL，仅在思考中自我说明）。\n",
			Examples: "1. 列举表：\n" +
				"   - 用户：\"这个 Hive 里有哪些订单相关的表？\"\n" +
				"   - 你：调用 list_tables，筛选名称中包含 \"order\" 的表，然后用自然语言总结。\n" +
				"2. 查看结构与分区：\n" +
				"   - 用户：\"看看 dwd_order_detail 这张表的字段。\"\n" +
				"   - 你：调用 describe_table（table_name=\"dwd_order_detail\"），重点说明主键/业务键、金额字段、时间字段和分区字段（如 dt）。\n" +
				"3. 只读查询：\n" +
				"   - 用户：\"统计最近 7 天按渠道的订单数和金额。\"\n" +
				"   - 你：在思考中说明将查询 dwd_order_detail，按 dt 限定最近 7 天，按渠道分组聚合，然后通过 execute_read 执行 SELECT，并用中文总结结果。\n",
		},
	}
}

func elasticsearchDescriptor() *TypeDescriptor {
	return &TypeDescriptor{
		Type: "elasticsearch",
		Tools: []ToolDef{
			{Name: ToolListTables, Description: "List indices in the current Elasticsearch datasource. Optional keyword: filter to indices whose name contains the keyword (fuzzy search). Returns index names and optional comments."},
			{Name: ToolDescribeTable, Description: "Describe index mapping (fields and types) in the current datasource."},
			{Name: ToolExecuteRead, Description: "Execute a read-only query: request body must be {\"query\":{...}} (e.g. {\"query\":{\"ids\":{\"values\":[\"id1\"]}}}). Optional index: target index or pattern. For _id lookup use ids or term query, not match."},
			// execute_write 不在 ES 默认能力列表，描述符中不包含；若未来开放则与 mysql 一致即可
		},
		Prompt: struct {
			Intro         string
			ToolSummaries string
			Workflow      string
			Examples      string
		}{
			Intro: "你是一个安全可靠的数据查询助手，负责通过工具访问 Elasticsearch 数据源，帮助用户用自然语言完成数据的「列举、结构查看、只读查询，以及在允许时的写/改」任务。\n\n",
			ToolSummaries: "- list_tables：列举的是**逻辑表（索引模式）**，如 vm-manager-* 表示同一模式的多个日期索引聚合；可选 keyword 模糊过滤。\n" +
				"- describe_table：入参可为模式名（如 vm-manager-*）或具体索引名；返回 mapping 与 Comment 中的**时间约定**（按日滚动时查某日数据应用对应索引或 pattern）。\n" +
				"- execute_read：传入 **Search 请求体**，必须是 **{\"query\":{...}}** 包裹（如 {\"query\":{\"ids\":{\"values\":[\"id1\"]}}} 或 {\"query\":{\"term\":{\"_id\":\"id1\"}}}），不能只传 {\"ids\":...}。可选 index 参数指定目标索引或 pattern。按 _id 查用 ids/term，不要用 match。\n",
			Workflow: "1. 探索阶段：用 list_tables 看逻辑表（索引模式），用 describe_table 看某模式或具体索引的 mapping 与时间约定。\n" +
				"2. 计划阶段：根据用户问题与 mapping，决定查哪个索引或 pattern；**时间序列查询需根据时间选索引**（见 describe_table 的 Comment）。\n" +
				"3. 只读查询阶段：\n" +
				"   - 使用 execute_read，必要时传 **index** 参数指定目标索引或 pattern；body 为 {\"query\":{...}} 的 Search 请求体（可按业务字段用 match、range、aggs 等；**仅当按文档 _id 精确查时**用 ids 或 term，勿用 match）。优先返回汇总再补充明细。\n" +
				"   - **形如 数字_字母数字 的值**（如 4103_3mrtug0l92h7）且用户未明确说「字段 M」或「列 M」时，视为**文档 _id**，用 {\"query\":{\"ids\":{\"values\":[\"该值\"]}}} 或 {\"query\":{\"term\":{\"_id\":\"该值\"}}}，**不要**用 {\"term\":{\"M\":\"该值\"}}。\n" +
				"   - 若首次查询因索引名错误（如用了 backend-vm_manager 而应为 backend-vm_manager-*）或 query 用错未命中，**应修正 index 与 query 后再次调用 execute_read**，不要仅用文字说明「请用 xxx」就结束；直到拿到工具返回结果再回复用户。\n" +
				"   - 查询前在思考中说明目标索引与意图。\n",
			Examples: "1. 列举索引：\n" +
				"   - 用户：\"有哪些索引？\"\n" +
				"   - 你：调用 list_tables，然后用自然语言总结索引名称与用途。\n" +
				"2. 查看 mapping：\n" +
				"   - 用户：\"帮我看看 logs 索引的字段。\"\n" +
				"   - 你：调用 describe_table（table_name=\"logs\"），并解释关键字段类型与含义。\n" +
				"3. 只读查询：\n" +
				"   - 用户：\"查一下 logs 里最近 10 条。\"\n" +
				"   - 你：在思考中说明将查询 logs 索引、size=10，然后通过 execute_read 传入 Search 请求体 JSON（如 {\"size\":10,\"query\":{\"match_all\":{}}}），最后用中文总结结果。\n" +
				"4. 按业务字段查询（如字段 M）：\n" +
				"   - 用户：\"查 backend-vm_manager 里 M 为 4103 的记录。\"\n" +
				"   - 你：传 index=backend-vm_manager-*（或具体日期索引），body 为 {\"query\":{\"term\":{\"M\":\"4103\"}}}，返回并总结。这是**业务字段 M**，不是文档 _id。\n" +
				"5. 按文档 _id 查询（勿与字段 M 混淆）：\n" +
				"   - 用户：\"查 id 为 4103_3mrtug0l92h7 的记录\" 或只给出一串 4103_3mrtug0l92h7。\n" +
				"   - 你：传 index=backend-vm_manager-*，body 为 {\"query\":{\"ids\":{\"values\":[\"4103_3mrtug0l92h7\"]}}} 或 {\"query\":{\"term\":{\"_id\":\"4103_3mrtug0l92h7\"}}}，**不要**用 term 查字段 M。若首次未传 index 或索引名写错，修正后再次调用 execute_read 再回复。\n",
		},
	}
}

func mongodbDescriptor() *TypeDescriptor {
	return &TypeDescriptor{
		Type: "mongodb",
		Tools: []ToolDef{
			{Name: ToolListTables, Description: "List collections in the current MongoDB datasource. Optional keyword: filter to collections whose name contains the keyword. Returns collection names and optional comments."},
			{Name: ToolDescribeTable, Description: "Describe collection structure in the current datasource. Returns sampled top-level fields and rough types."},
			{Name: ToolExecuteRead, Description: "Execute a read-only MongoDB find query. DSL must be a JSON string like {\"collection\":\"users\",\"filter\":{\"status\":\"active\"},\"limit\":50}."},
		},
		Prompt: struct {
			Intro         string
			ToolSummaries string
			Workflow      string
			Examples      string
		}{
			Intro: "你是一个安全可靠的数据查询助手，负责通过工具访问 MongoDB 数据源，帮助用户用自然语言完成数据的「列举、结构查看、只读查询」任务（写/改能力需单独开放）。\n\n",
			ToolSummaries: "- list_tables：列举当前数据库中的集合（collections），可选 keyword 模糊过滤集合名。\n" +
				"- describe_table：查看指定集合的示例字段（基于采样文档的顶层字段），用于在编写查询前理解文档结构。\n" +
				"- execute_read：执行只读查询，DSL 为 JSON 字符串，例如 {\"collection\":\"users\",\"filter\":{\"status\":\"active\"},\"limit\":50}，相当于对指定集合做 find 查询。\n",
			Workflow: "1. 探索阶段：当你不了解数据库结构时，先使用 list_tables 获取有哪些集合，再针对关键集合使用 describe_table 了解字段与文档形态。\n" +
				"2. 计划阶段：根据用户问题和集合结构，在「思考」中用自然语言说明你打算查询哪些集合、使用哪些字段作为过滤条件与投影。\n" +
				"3. 只读查询阶段：\n" +
				"   - 使用 execute_read，构造 JSON DSL：至少包含 collection 字段，可选 filter（对象）、limit（整数）、projection（对象，如 {\"field\":1}）、sort（对象，如 {\"created_at\":-1}）。\n" +
				"   - 优先返回聚合或关键结论（如数量、按某字段分布），必要时再补充明细文档示例，不要一次性返回过多行。\n" +
				"   - 查询前在思考中简要说明你将使用的集合与过滤条件。\n",
			Examples: "1. 列举集合：\n" +
				"   - 用户：\"这个 MongoDB 里有哪些和用户相关的集合？\"\n" +
				"   - 你：调用 list_tables，筛选名称中包含 \"user\" 的集合，然后用自然语言总结。\n" +
				"2. 查看结构：\n" +
				"   - 用户：\"帮我看看 users 集合的字段。\"\n" +
				"   - 你：调用 describe_table（table_name=\"users\"），基于示例文档字段解释关键字段含义。\n" +
				"3. 只读查询：\n" +
				"   - 用户：\"查出最近注册的 10 个活跃用户。\"\n" +
				"   - 你：在思考中说明将查询 users 集合，按 created_at 降序、status=active，并限制 10 条，然后通过 execute_read 传入 JSON DSL（如 {\"collection\":\"users\",\"filter\":{\"status\":\"active\"},\"sort\":{\"created_at\":-1},\"limit\":10}），最后用中文总结结果。\n",
		},
	}
}

// DataQueryPromptConfig 配置数据查询 Agent 的系统提示。
type DataQueryPromptConfig struct {
	// DatasourceType 如 "mysql"、"postgres"。仅用于文案描述。
	DatasourceType string
	// AllowWrite 为 false 时仅允许只读工具（不应调用 execute_write）。
	AllowWrite bool
}

// BuildDataQuerySystemPrompt 构造数据查询 Agent 的系统级提示，由类型描述符驱动，无按类型 if/else。
func BuildDataQuerySystemPrompt(cfg DataQueryPromptConfig) string {
	ds := cfg.DatasourceType
	if ds == "" {
		ds = "mysql"
	}
	desc := GetDescriptor(ds)

	var b strings.Builder
	b.WriteString(desc.Prompt.Intro)

	b.WriteString("【总体原则】\n")
	b.WriteString("1. 严格遵守只读/写改边界：除非明确允许写/改且用户有强烈意图，否则不要修改数据；**严禁编造、臆造或假设任何查询结果**：仅能依据工具返回的真实数据回答，工具返回空即明确告知「未查到数据」。\n")
	if !cfg.AllowWrite {
		b.WriteString("2. 当前运行在「只读模式」，你**禁止调用**任何会修改数据的工具（如 execute_write）。若用户请求写/改，请解释当前仅支持查询，并给出只读替代方案。\n")
	} else {
		b.WriteString("2. 写/改操作必须经过**两阶段确认**：先提议（生成确认 token），等待用户基于该 token 进行自然语言确认，只有在用户明确同意且携带 token 时才真正执行。\n")
	}
	b.WriteString("3. 始终用简洁的中文回答用户，必要时在结果后面补充简短的业务含义解释。\n\n")

	b.WriteString("【可用工具与用途】\n")
	b.WriteString(desc.Prompt.ToolSummaries)
	if cfg.AllowWrite {
		b.WriteString("- execute_write：用于 INSERT/UPDATE/DELETE 等写/改操作，只能在用户已经明确确认且你持有有效 confirm_token 时使用。\n")
	} else {
		b.WriteString("- （禁用）execute_write：当前环境为只读模式，你不应调用该工具。\n")
	}
	b.WriteString("\n")

	b.WriteString("【推荐工作流（ReAct 思考→行动→观察）】\n")
	b.WriteString(desc.Prompt.Workflow)
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
	b.WriteString("4. **若工具返回的结果为空（0 条）**：必须明确告知用户「未查到数据」或「当前无匹配结果」，不得编造、臆造或假设任何一条记录或字段内容；可简要说明可能原因（如条件过严、索引/表名错误）或建议下一步（如先 list_tables/describe_table 再重试）。\n")
	b.WriteString("5. 若查询结果非空但你不确定含义，请直接说明，并给出可能的原因或下一步排查建议。\n\n")

	b.WriteString("【示例场景】\n")
	b.WriteString(desc.Prompt.Examples)
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
	// DatasourceTypes 可选：datasource_id -> type（如 "mysql"、"elasticsearch"），用于按请求数据源生成对应系统提示与能力列表。
	DatasourceTypes map[string]string
	// ToolCapabilitiesByType 可选：datasource_type -> 支持的工具名列表；为 nil 时使用 DefaultToolCapabilitiesByType。
	ToolCapabilitiesByType map[string][]string
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

	// 构建数据源 Registry（支持 mysql、hive、elasticsearch、mongodb 类型）。
	dsReg := datasource.NewRegistry()
	datasource.RegisterMySQL(dsReg)
	datasource.RegisterHive(dsReg)
	datasource.RegisterElasticsearch(dsReg)
	datasource.RegisterMongoDB(dsReg)
	idToType := make(map[string]string)
	for _, ds := range cfg.DataSources {
		if ds.Type == "" {
			ds.Type = "mysql"
		}
		idToType[ds.ID] = ds.Type
		if _, err := dsReg.Register(ds); err != nil {
			return nil, fmt.Errorf("dataquery: register datasource %s: %w", ds.ID, err)
		}
	}

	store := metadata.NewInMemoryStore(nil)
	exec := executor.NewMultiExecutor(dsReg, executor.NewMySQLExecutor(dsReg), executor.NewESExecutor(dsReg), executor.NewMongoExecutor(dsReg))

	dqCfg := DataQueryConfig{
		DatasourceRegistry:     dsReg,
		MetadataStore:          store,
		Exec:                   exec,
		Checker:                auth.PermissiveChecker{},
		DefaultDatasourceID:    cfg.DefaultDatasourceID,
		DatasourceTypes:        idToType,
		AllowWrite:             cfg.DataAllowWrite,
		MaxReActSteps:          20,
		DefaultReadTimeoutSec:  0,
		DefaultReadMaxRows:     0,
		WriteConfirmTTLSeconds: 300,
		DefaultWriteTimeoutSec: 0,
	}

	return NewDataQueryHandler(m, mem, dqCfg, mws...), nil
}

// registerDataQueryTools 按类型描述符将工具注册到 reg，描述符中带 Description 的会覆盖 tool 包默认描述。
func registerDataQueryTools(reg *tool.Registry, cfg DataQueryConfig, desc *TypeDescriptor) {
	if reg == nil || desc == nil {
		return
	}
	for _, td := range desc.Tools {
		if td.Name == ToolExecuteWrite && !cfg.AllowWrite {
			continue
		}
		var opts *tool.RegisterToolOptions
		if td.Description != "" {
			opts = &tool.RegisterToolOptions{Description: td.Description}
		}
		switch td.Name {
		case ToolListTables:
			_ = tool.RegisterListTablesTool(reg, &tool.ListTablesConfig{
				Store:               cfg.MetadataStore,
				Registry:            cfg.DatasourceRegistry,
				DefaultDatasourceID: cfg.DefaultDatasourceID,
			}, opts)
		case ToolDescribeTable:
			_ = tool.RegisterDescribeTableTool(reg, &tool.DescribeTableConfig{
				Store:               cfg.MetadataStore,
				Registry:            cfg.DatasourceRegistry,
				DefaultDatasourceID: cfg.DefaultDatasourceID,
			}, opts)
		case ToolExecuteRead:
			_ = tool.RegisterExecuteReadTool(reg, &tool.ExecuteReadConfig{
				Exec:                cfg.Exec,
				DefaultDatasourceID: cfg.DefaultDatasourceID,
				DefaultTimeoutSec:   cfg.DefaultReadTimeoutSec,
				DefaultMaxRows:      cfg.DefaultReadMaxRows,
			}, opts)
		case ToolExecuteWrite:
			if !cfg.AllowWrite {
				continue
			}
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
			}, opts)
		}
	}
}

// NewDataQueryHandler 构建一个专用于数据查询的 ReAct Agent 处理器。
// 工具按请求的数据源类型动态注册（仅注册该类型支持的工具），系统提示按 DatasourceType 分支注入。
func NewDataQueryHandler(m model.Model, mem memory.Memory, cfg DataQueryConfig, mws ...middleware.Middleware) middleware.Handler {
	if cfg.DatasourceRegistry == nil || cfg.MetadataStore == nil || cfg.Exec == nil {
		panic("NewDataQueryHandler: DatasourceRegistry, MetadataStore and Exec are required")
	}
	if cfg.MaxReActSteps <= 0 {
		cfg.MaxReActSteps = 4
	}
	if cfg.WriteConfirmTTLSeconds <= 0 {
		cfg.WriteConfirmTTLSeconds = 300
	}

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

		// 按当前请求的数据源 ID 解析类型，用于取描述符（工具能力 + 提示片段）。
		datasourceType := "mysql"
		if cfg.DatasourceTypes != nil && datasourceID != "" {
			if t := cfg.DatasourceTypes[datasourceID]; t != "" {
				datasourceType = t
			}
		}
		descriptor := GetDescriptor(datasourceType)
		reg := tool.NewRegistry()
		registerDataQueryTools(reg, cfg, descriptor)
		react := agent.NewReActAgent(m, mem, reg, agent.WithReActMaxSteps(cfg.MaxReActSteps))

		promptCfg := DataQueryPromptConfig{
			DatasourceType: datasourceType,
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
		llmReq := *req
		llmReq.Messages = append([]model.Message{
			{Role: "system", Content: sys},
		}, req.Messages...)

		// 标记 agent_name 以便 metrics 中区分 dataquery 请求。
		if llmReq.Metadata == nil {
			llmReq.Metadata = make(map[string]any)
		}
		if _, ok := llmReq.Metadata["agent_name"]; !ok {
			llmReq.Metadata["agent_name"] = "dataquery"
		}

		// 将 RequestID 注入 ctx，便于下游工具（如 execute_write）写审计事件时使用。
		if llmReq.RequestID != "" {
			ctx = context.WithValue(ctx, "request_id", llmReq.RequestID)
		}

		return react.Run(ctx, &llmReq)
	}

	return middleware.Chain(final, mws...)
}
