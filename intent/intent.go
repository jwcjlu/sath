package intent

// Intent 表示用户意图类型。
type Intent string

const (
	IntentQuery    Intent = "query"
	IntentInsert   Intent = "insert"
	IntentUpdate   Intent = "update"
	IntentDelete   Intent = "delete"
	IntentMetadata Intent = "metadata"
	IntentRewrite  Intent = "rewrite"
)

// Condition 表示一条条件（字段、操作符、值）。
type Condition struct {
	Field string `json:"field"`
	Op    string `json:"op"`    // eq, ne, gt, gte, lt, lte, like, in
	Value any    `json:"value"`
}

// Aggregation 表示聚合：函数名 + 字段。
type Aggregation struct {
	Func  string `json:"func"`  // sum, avg, count, min, max
	Field string `json:"field"`
}

// OrderByItem 排序项。
type OrderByItem struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc"`
}

// Entities 从自然语言中抽取的实体，供 DSL 生成使用。
type Entities struct {
	DatasourceID string          `json:"datasource_id,omitempty"`
	Schema       string          `json:"schema,omitempty"`
	Table        string          `json:"table,omitempty"`
	Columns      []string        `json:"columns,omitempty"`
	Conditions   []Condition     `json:"conditions,omitempty"`
	Aggregations []Aggregation    `json:"aggregations,omitempty"`
	OrderBy      []OrderByItem   `json:"order_by,omitempty"`
	Limit        int             `json:"limit,omitempty"`
	Offset       int             `json:"offset,omitempty"`
	SetClause    map[string]any  `json:"set_clause,omitempty"` // UPDATE 的 SET 列=值
	Values       map[string]any  `json:"values,omitempty"`    // INSERT 的列=值
}

// ParsedInput 意图识别输出：意图 + 实体 + 原始问句与可选上下文。
type ParsedInput struct {
	Intent       Intent    `json:"intent"`
	Entities     Entities  `json:"entities"`
	RawNL        string    `json:"raw_nl,omitempty"`
	PreviousDSL  string    `json:"previous_dsl,omitempty"`
	UncertainFields []string `json:"uncertain_fields,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}
