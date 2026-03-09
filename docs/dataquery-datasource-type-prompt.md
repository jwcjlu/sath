# 方案：按数据源类型生成系统提示（支持 MySQL / Elasticsearch）

## 问题

`templates/dataquery.go` 第 281–285 行将 `DatasourceType` 写死为 `"mysql"`，导致使用 Elasticsearch 数据源时，系统提示仍描述「SQL / 表 / SELECT」，与 ES 的「索引 / mapping / Search 请求体」不一致，影响模型行为。

## 目标

- 根据**当前请求使用的数据源**决定系统提示中的「数据源类型」及表述。
- 支持 MySQL 与 Elasticsearch；后续可扩展其他类型（如 postgres）。

## 方案概述

1. **配置层**：在 `DataQueryConfig` 中增加「数据源 ID → 类型」映射，并在 `NewDataQueryHandlerFromConfig` 中从 `config.DataSources` 填充。
2. **请求层**：在处理每次请求时，用当前 `datasource_id` 查上述映射得到 `datasourceType`，再传给 `BuildDataQuerySystemPrompt`。
3. **提示层**：在 `BuildDataQuerySystemPrompt` 中根据 `DatasourceType` 分支，对 Elasticsearch 使用「索引 / mapping / execute_read 为 Search 请求体 JSON」等表述，对 MySQL（或默认）保持现有 SQL 表述。

---

## 1. 配置与请求解析

- **DataQueryConfig** 增加可选字段：
  - `DatasourceTypes map[string]string`  
  - 含义：`datasource_id -> type`，例如 `{"main": "elasticsearch"}`。  
  - 为 nil 或未命中时，沿用当前默认行为（视为 SQL/MySQL）。

- **NewDataQueryHandlerFromConfig** 中，在遍历 `cfg.DataSources` 注册完数据源后，构造该 map：
  - `idToType := make(map[string]string)`
  - 对每个 `ds := range cfg.DataSources`：`idToType[ds.ID] = ds.Type`（若 `ds.Type` 为空则用 `"mysql"`，与现有逻辑一致）
  - 将 `idToType` 赋给 `dqCfg.DatasourceTypes`。

- **Handler 内（final 闭包）** 构造 prompt 时：
  - 已具备 `datasourceID`（来自 Metadata 或 DefaultDatasourceID）。
  - `datasourceType := "mysql"`（默认）
  - 若 `cfg.DatasourceTypes != nil && datasourceID != ""` 且 `cfg.DatasourceTypes[datasourceID] != ""`，则 `datasourceType = cfg.DatasourceTypes[datasourceID]`。
  - 将 `datasourceType` 传入 `DataQueryPromptConfig.DatasourceType`，不再写死 `"mysql"`。

这样无需改 `datasource.Registry`，类型完全由配置决定，且与现有「按 ID 选数据源」一致。

---

## 2. 提示文案按类型分支

- **BuildDataQuerySystemPrompt(cfg)** 中根据 `cfg.DatasourceType` 分支（不改变函数签名）：
  - **elasticsearch**（或统一将 `"es"` 视为 `"elasticsearch"`）：
    - 首句：如「你是一个安全可靠的数据查询助手，负责通过工具访问 **Elasticsearch** 数据源」。
    - 工具说明中：
      - list_tables：列举当前数据源中的**索引**及简要说明。
      - describe_table：查看某**索引**的 **mapping（字段）**、类型等。
      - execute_read：执行只读查询，传入 **Search API 的请求体 JSON**（如 query、size、_source），返回匹配文档或聚合结果。
    - 工作流与示例中：用「索引」「mapping」「Search 请求体」替代「表」「列」「SELECT」；可保留「先 list_tables / describe_table 再 execute_read」的流程。
  - **默认（mysql / sql 及其他）**：
    - 保持现有全文：SQL、表、列、SELECT、execute_read 执行 SELECT 等。

- 实现方式建议：
  - 在 `BuildDataQuerySystemPrompt` 内用 `switch cfg.DatasourceType` 或 `if cfg.DatasourceType == "elasticsearch" || cfg.DatasourceType == "es"` 分支；
  - 或将「首段 + 工具说明 + 工作流」按类型拆成若干小函数/常量，减少重复。

---

## 3. 验收

- 使用 **MySQL** 配置与 datasource_id 时，系统提示仍为当前 SQL/表/SELECT 表述，行为与现在一致。
- 使用 **Elasticsearch** 配置与 datasource_id 时，系统提示中出现 Elasticsearch、索引、mapping、Search 请求体等表述，且 list_tables / describe_table / execute_read 的说明与 ES 语义一致。
- 未配置 `DatasourceTypes` 或请求的 datasource_id 不在 map 中时，默认按 MySQL/SQL 提示，不报错。

---

## 4. 小结

| 项目       | 做法 |
|------------|------|
| 类型来源   | 配置中的 `data_sources[].id` 与 `type`，组装为 `DataQueryConfig.DatasourceTypes`。 |
| 请求时解析 | 用当前 `datasource_id` 查 `DatasourceTypes`，得到 `datasourceType` 再生成 prompt。 |
| 提示分支   | `BuildDataQuerySystemPrompt` 内按 `DatasourceType == "elasticsearch"`（或 `"es"`）走 ES 文案，否则走现有 SQL 文案。 |

评审通过后可按此方案修改 `templates/dataquery.go`（与可选的小幅提示重构）。

---

## 5. 评审意见（合理性 & 优雅性）

### 合理性：✅ 成立

- **问题对准**：写死 `"mysql"` 导致 ES 请求也拿到 SQL 向 prompt，方案改为「按当前请求的 datasource_id 查类型 → 再生成 prompt」，直接解决。
- **数据一致**：类型来自「注册数据源时用的配置」`data_sources[].type`，与 Registry 里实际注册的类型一致，不会出现「配置是 ES、提示是 MySQL」的分叉。
- **默认与兜底**：未配置 `DatasourceTypes` 或 datasource_id 查不到时回退到 MySQL/SQL，兼容现有调用与单数据源场景，不引入破坏性变更。
- **边界清晰**：谁提供类型（配置）、谁消费类型（prompt 生成）、何时解析（每次请求）都明确，没有隐式假设。

### 优雅性：✅ 可接受，有小改进空间

- **不引入额外抽象**：没有为「多数据源类型」单独再建一层接口或策略表，仅用 `map[string]string` + 分支，和当前架构匹配，改动面小。
- **单一数据源**：类型由「当前请求选中的数据源」决定，而不是「所有配置过的类型」混合，语义简单，不会出现「一个 prompt 里既讲 SQL 又讲 ES」的混乱。
- **可维护性**：新增一种数据源（如 postgres）时，只需在配置里加 type、在 `BuildDataQuerySystemPrompt` 里加一个分支（或复用「默认 SQL」），成本可控。

**可考虑的改进（非必须）**：

1. **类型归一化**：在查 `DatasourceTypes[datasourceID]` 后做一次归一化（如 `"es"` → `"elasticsearch"`），再传给 `BuildDataQuerySystemPrompt`，这样 prompt 层只认一种写法，减少魔法字符串。
2. **提示结构**：若后续类型增多，可在 `BuildDataQuerySystemPrompt` 内把「首段 / 工具说明 / 工作流 / 示例」按类型抽成小函数或常量，避免一个超长 `switch`；当前只有 2 类时，直接 `if ds == "elasticsearch" { ... } else { ... }` 即可，不必过度抽象。
3. **文档约定**：在代码或注释里约定 `DatasourceTypes` 的 key 与 `data_sources[].id` 一致、value 与 `data_sources[].type` 一致（如 `mysql` / `elasticsearch`），方便后续扩展和排查。

### 结论

方案**合理且足够优雅**：满足「按数据源类型生成对应系统提示」的目标，不破坏现有行为，扩展路径清晰。建议采纳并按文档实现；上述改进可在实现时按需选用。
