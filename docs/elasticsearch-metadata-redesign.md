# Elasticsearch 元数据与时间序列索引 — 方案设计

本文档针对当前 ES 元数据拉取与使用中的三个问题，提出重新设计方案，**约定方案评审通过后再实施开发**。

---

## 一、问题陈述

| 问题 | 表现 | 影响 |
|------|------|------|
| **1. 索引很多时拉取元数据很慢** | 当前对每个索引单独请求 `_mapping`，索引数量 N 即 N 次请求；且 Refresh 时一次性拉全量。 | 数百/数千个索引时耗时长、易超时，list_tables 体验差。 |
| **2. 大量索引共用同一 mapping** | 如 `vm-manager-2026.01.02` 与 `vm-manager-2026.01.03` 仅日期后缀不同，mapping 一致，但当前按「一索引一 Table」存储，重复拉取、重复存储。 | 浪费请求与内存，且 list_tables 列表过长、噪音大。 |
| **3. 时间序列查询需知道「数据在哪个索引」** | 按日/按小时滚动的索引，用户问「查 1 月 2 日的数据」时，Agent 需要知道应查 `vm-manager-2026.01.02` 或使用 pattern `vm-manager-*` 并配合时间范围。 | 当前元数据只暴露索引名列表，无「时间范围 → 索引」的约定或提示，模型难以做对。 |

---

## 二、设计目标

1. **减少拉取耗时与请求数**：刷新时尽量少调 ES，或只拉「索引列表」等轻量信息，mapping 按需或按组拉取。
2. **按「索引模式」聚合，mapping 复用**：将同模版/同模式的索引归为「逻辑表」（如 `vm-manager-*`），每逻辑表只存一份 mapping，list_tables 返回逻辑表而非上千个具体索引名。
3. **支持时间序列可发现性**：让 Agent 能知道某逻辑表是「按日/按小时」滚动、日期在索引名中的位置与格式，从而在 execute_read 时选择正确索引或 pattern。

---

## 三、方案概述

### 3.1 核心思路

- **索引分组（Index Pattern）**：根据索引名将多个索引归纳为「索引模式」逻辑表（如 `vm-manager-*`），一个模式对应 Schema 中的一个 Table（逻辑表）。
- **Mapping 按组只取一份**：每个模式只对「代表索引」拉一次 mapping（或使用 Index Template API 取模板 mapping），该模式的 Table.Columns 共用这一份 mapping。
- **刷新阶段只拉索引列表 + 分组**：Refresh 时仅调用 `_cat/indices`（及可选 `_index_template`），在内存中做分组与代表索引选取，**不**对每个索引调 `_mapping`；如需某模式的 mapping，再按需拉取或从模板填充。
- **时间序列约定可描述**：在逻辑表的 Comment（或扩展字段）中描述「时间滚动规则」（如「按日，后缀 YYYY.MM.DD」），并在系统提示/describe_table 中说明「查某日数据请用索引 xxx 或 pattern vm-manager-* 并限定时间范围」。

### 3.2 与现有抽象的兼容

- **Schema / Table / Column**：仍使用现有 `metadata.Schema`、`Table`、`Column`。Table 表示「逻辑表」（一个索引模式），Table.Name 为模式名（如 `vm-manager-*`），Table.Comment 可存「示例索引、时间约定」等说明；Table.Columns 为该模式的一份 mapping。
- **list_tables**：返回「逻辑表」列表（模式名 + 可选 comment），不再返回成千上万个具体索引名；可选保留「带 keyword 过滤」在模式名上做模糊匹配。
- **describe_table**：入参可为模式名（如 `vm-manager-*`）或具体索引名；若为模式则返回该模式的 mapping（及时间约定说明）；若为具体索引则按需解析其所属模式并返回同一份 mapping，或单独拉该索引 mapping 作为兜底。
- **execute_read**：ES 执行层支持「指定索引/pattern」：execute_read 增加可选参数 `index` / `indices`（或 DSL 顶层支持 index），执行时请求发往 `GET /<index|pattern>/_search`，使时间序列查询能限定到某日索引或 pattern。

---

## 四、详细设计

### 4.1 索引分组策略（Index Pattern 识别）

- **输入**：`_cat/indices` 得到的索引名列表（已过滤系统索引）。
- **分组规则**（可配置、可扩展）：
  - **按日期/时间后缀分组**：若索引名符合 `^(.+)[.-](\d{4}[.-]\d{2}[.-]\d{2})$` 或带小时等，将前缀视为模式 base，模式名为 `base*`（如 `vm-manager-*`），同组内选一个「代表索引」用于后续拉 mapping（如最新一个）。
  - **按 Index Template 分组**（可选）：若集群有 `_index_template`，可按 template 名或 index_pattern 分组，mapping 直接来自模板定义，无需再调每个索引的 `_mapping`。
  - **兜底**：无法归组的索引，可单独成「逻辑表」（Name = 该索引名），或归入一个「其它」组。
- **配置**：分组规则可放在 metadata 包配置或 datasource 配置中（如「时间后缀正则」「是否启用 template 分组」），便于不同集群约定一致。

### 4.2 刷新流程（Refresh）

1. 调用 `_cat/indices` 获取索引列表（与现有一致）。
2. 在内存中对索引名做**分组**，得到「模式名 → 代表索引、该组下所有索引名（可选）」。
3. **不**在此阶段对每个索引调用 `_mapping`。
4. 构建 Schema：每个模式对应一个 Table（Name = 模式名，如 `vm-manager-*`）；Table.Comment 可写入「示例索引：vm-manager-2026.01.02, vm-manager-2026.01.03」「时间约定：按日滚动，后缀 YYYY.MM.DD」等；Table.Columns 此时可为空。
5. **Mapping 按需加载**：
   - **方案 A（推荐）**：describe_table 被调用时，若该 Table 的 Columns 为空，则对该模式的「代表索引」请求一次 `_mapping`，填回 Schema 中该 Table 的 Columns，并缓存（当前 Store 已缓存整个 Schema，故下次 describe_table 同一模式不再请求）。
   - **方案 B**：Refresh 时对每个「模式」只请求代表索引的 mapping 一次（模式数通常远小于索引数），立即填充 Table.Columns。

方案 A 使「仅 list_tables、不 describe」的场景零 mapping 请求；方案 B 实现更简单，list_tables 即能看到列信息。可先实现 B，后续再支持 A（懒加载）。

### 4.3 时间序列约定描述

- 在分组时，若识别出「日期/时间后缀」，可在 Table.Comment 中写入约定文案，例如：
  - `时间序列：按日滚动，索引后缀为 YYYY.MM.DD。查询某日数据请使用索引 <base>-YYYY.MM.DD 或 pattern <base>-*，并在 query 中限定时间范围。`
- 系统提示或类型描述符中可增加一句：**时间序列索引的查询需根据时间选择目标索引或 pattern，describe_table 的 Comment 会说明该逻辑表的时间约定。**

### 4.4 execute_read 与「目标索引」

- **现状**：ES 执行器使用 `client.Search(WithBody(...))`，未传 index，即请求发往 `_search`（全部索引）。
- **扩展**：execute_read 工具增加可选参数 `index`（string，可为单个索引名、多个索引逗号分隔、或 pattern 如 `vm-manager-*`）；ES 执行器在执行时若 params 中有 index，则使用 `Search.WithIndex(index)`，使请求发往 `GET /<index>/_search`，从而支持「查某日索引」或「查某 pattern」。
- 与现有 DSL 兼容：用户仍可在 body 中写 query；index 仅影响请求路径，不要求改 DSL 结构。

### 4.5 可选：Index Template API

- 若启用「按模板分组」，Refresh 时调用 `GET _index_template`（或 `_cat/templates`），得到 index_pattern 与模板 mapping；分组时优先按 index_pattern 将索引归入某模板，Table.Name 可用 index_pattern（如 `vm-manager-*`），Table.Columns 来自模板 mapping，无需再请求任何具体索引的 `_mapping`。
- 此能力依赖集群已配置 Index Template，可作为可选增强。

---

## 五、实施要点（评审通过后执行）

| 步骤 | 内容 |
|------|------|
| 1 | **metadata**：新增「索引分组」逻辑（正则或可配置规则），将索引名列表归纳为「模式名 → 代表索引、时间约定文案」；FetchSchemaElasticsearch 改为只拉 _cat/indices + 分组，每个模式只对代表索引拉一次 _mapping（或先不拉，见步骤 2）。 |
| 2 | **metadata**：Schema 中的 Table 表示「逻辑表」；Table.Name = 模式名；Table.Comment = 示例索引 + 时间约定说明；Table.Columns = 该模式一份 mapping。可选：describe_table 时懒加载 mapping（若 Columns 为空则请求代表索引 _mapping 并写回 Store）。 |
| 3 | **tool list_tables**：行为不变，仅返回 Schema.Tables（此时为逻辑表列表）；keyword 过滤作用在 Table.Name（模式名）上。 |
| 4 | **tool describe_table**：入参 table_name 可为模式名或具体索引名；若为具体索引名，先解析其所属模式，再返回该模式的 Table（含 mapping 与 comment）；若模式存在但 Columns 为空且支持懒加载，则先拉 mapping 再返回。 |
| 5 | **tool execute_read**：增加可选参数 `index`（或 `indices`）；ES 执行器在执行时若存在该参数，则使用 `Search.WithIndex(index)` 指定目标索引/pattern。 |
| 6 | **executor**：ES 执行器 Execute 接口从 opts.Params 或上下文中读取 index，并传入 Search API。 |
| 7 | **类型描述符/系统提示**：在 Elasticsearch 描述符中补充「时间序列索引」说明：list_tables 返回的是逻辑表（索引模式），describe_table 会说明时间约定，查询时通过 execute_read 的 index 参数指定目标索引或 pattern。 |

---

## 六、风险与取舍

- **分组规则与集群约定强相关**：不同集群命名不一，可能需要可配置正则或策略（如「只按日期后缀」「只按 template」）。建议先实现一种默认策略（如日期后缀），再通过配置扩展。
- **describe_table(table_name=具体索引)**：若用户传的是具体索引名而非模式名，需要「从索引名反推模式」（用同一分组规则），找到对应逻辑表并返回其 mapping；若该索引未落入任何组（兜底单索引），可单独拉该索引 mapping 返回。
- **兼容性**：现有使用「具体索引名」的 describe_table / execute_read 仍可支持：describe 时能解析到模式则返回模式 mapping，否则按单索引返回；execute_read 不传 index 时保持当前「_search」行为（全索引），传 index 时收窄到指定索引/pattern。

---

## 七、小结

| 问题 | 解决方式 |
|------|----------|
| 索引多时拉取慢 | 刷新只拉 _cat/indices + 分组，每模式只拉 1 次 mapping（或 describe 时懒加载），请求数从 O(N) 降为 O(模式数)。 |
| 同模版索引重复 mapping | 按索引模式聚合成逻辑表，每逻辑表存一份 mapping，list_tables 返回逻辑表列表。 |
| 时间序列不知道查哪个索引 | 逻辑表 Comment 描述时间约定；execute_read 支持 index 参数指定索引/pattern；提示词说明「根据时间选索引或 pattern」。 |

**约定：方案评审通过后再进入开发与接口实现。**
