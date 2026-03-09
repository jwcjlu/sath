package metadata

// Column 描述单个字段
type Column struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsNullable bool   `json:"is_nullable"`
	Comment    string `json:"comment,omitempty"`
}

// Table 描述一张表/视图
type Table struct {
	Name    string   `json:"name"`
	Comment string   `json:"comment,omitempty"`
	Columns []Column `json:"columns"`
}

// Schema 描述整个逻辑库
type Schema struct {
	// Name 通常对应数据库名
	Name   string  `json:"name"`
	Tables []Table `json:"tables"`
	// IndexToPattern 可选，仅 Elasticsearch 使用：具体索引名 -> 逻辑表名（索引模式，如 vm-manager-*）。
	// 用于 describe_table(table_name=具体索引名) 时解析到对应逻辑表。
	IndexToPattern map[string]string `json:"-"`
}
