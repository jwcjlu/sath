package metadata

// Schema 表示某数据源下的逻辑库/库级元数据（关系型以库为单位）。
type Schema struct {
	Name   string   `json:"name"`   // 库名
	Tables []*Table `json:"tables"` // 表列表
}

// Table 表示单张表的元数据。
type Table struct {
	Name    string    `json:"name"`
	Columns []*Column  `json:"columns"`
}

// Column 表示列信息。
type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`     // 数据库类型，如 VARCHAR(64), INT
	Nullable bool   `json:"nullable"`
	PrimaryKey bool `json:"primary_key"`
}
