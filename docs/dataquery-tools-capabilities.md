# 数据查询工具与数据源能力统一设计

## 现状

当前在 `NewDataQueryHandler` 中**固定注册**四类工具：`list_tables`、`describe_table`、`execute_read`、`execute_write`（写可选）。所有配置的数据源共用同一套工具，实际行为由**运行时**的 `datasource_id` 决定：MetadataStore / Exec 按数据源类型分发到 MySQL、Elasticsearch 等。

问题在于：并非所有数据源都具备「列举 / 结构 / 只读 / 写」四种能力。例如：

- 某 KV 存储可能只有「列举 key」+「读」，没有「表结构」概念；
- 某只读数据源不应出现 `execute_write`（已通过 AllowWrite 控制）；
- 未来若接入仅支持「执行查询」的 API，可能没有 list/describe。

因此需要一套**统一且可扩展**的约定：哪些数据源类型支持哪些工具、如何表达「不支持」、是否要按能力注册或按能力报错。

---

## 两种方案对比：执行时校验 vs 按类型动态注册

| 维度 | 执行时校验（统一注册） | 按数据源类型动态注册（推荐） |
|------|------------------------|------------------------------|
| **做法** | 始终注册全部工具；在工具 Execute 内根据 datasource_id 查能力，不支持则返回明确错误。 | 每个请求根据当前 datasource 的**类型**，只注册该类型支持的工具，再创建/使用 ReActAgent 跑本轮。 |
| **模型侧** | 会看到 list/describe/read/write 等全部工具，可能调用到不支持的工具再收到错误。 | 只看到当前类型支持的工具，不会出现「调了再报不支持」的体验。 |
| **语义** | 「工具都在，但部分对当前数据源不可用」。 | 「当前数据源就这些能力」，与类型一一对应，更清晰。 |
| **实现成本** | 小：在现有工具 Execute 里加一段能力查询 +  early return。 | 中：需要能力矩阵 + 按类型只注册子集的注册函数；每请求建一次 Registry（与可选的一次 Agent）。 |
| **扩展性** | 新类型只需在能力矩阵里声明，工具内自动校验。 | 新类型同样只改能力矩阵，注册逻辑根据矩阵决定注册哪些工具，扩展点单一。 |
| **与 Agent 的耦合** | 不依赖 Agent 接口。 | 不修改 Agent 接口：仍用 `NewReActAgent(m, mem, reg, opts)`；只是每请求用「按类型建好的 reg」新建一个 ReActAgent 再 Run，Agent 本身无感知。 |

**结论**：**按数据源类型在请求时动态注册工具更优雅**——模型只看到「当前数据源真正支持」的工具，语义清晰、无需「先调用再报错」；实现上仅需在 Handler 内按请求解析类型 → 按能力矩阵建 Registry → 用该 Registry 建 ReActAgent 并 Run，不改 Agent 包接口。

---

## 设计目标

1. **统一抽象**：工具层保持「列举 / 结构 / 只读 / 写」四类能力，语义统一（list / describe / read / write），具体数据源在底层实现或明确声明不支持。
2. **可扩展**：新增数据源类型时，只需声明该类型支持哪些能力，无需改 ReAct 循环或 Agent 接口。
3. **优雅呈现**：模型侧只看到当前数据源支持的工具（推荐：按类型动态注册），避免「调了再报不支持」的体验。
4. **与现有架构兼容**：动态注册在 Handler 内按请求构建 Registry 并传入现有 `NewReActAgent(m, mem, reg, opts)`，无需改 Agent 的 Run 签名。

---

## 推荐方案：能力矩阵 + 按请求动态注册工具

### 1. 能力与工具的对应关系

约定四种能力与现有工具的对应关系（不变）：

| 能力 | 工具名 | 说明 |
|------|--------|------|
| ListObjects | list_tables | 列举表/索引/集合等 |
| DescribeObject | describe_table | 查看单表/索引的结构（列/mapping） |
| ExecuteRead | execute_read | 只读查询（SQL / Search 等） |
| ExecuteWrite | execute_write | 写/改（可选，且受 AllowWrite 控制） |

### 2. 按数据源类型声明能力

在**配置或常量**中维护「数据源类型 → 支持的能力列表」：

- **mysql**：ListObjects, DescribeObject, ExecuteRead, ExecuteWrite（与现有一致）
- **elasticsearch**：ListObjects, DescribeObject, ExecuteRead（当前 ES 示例未开放写，若开放可加 ExecuteWrite）
- 未来 **postgres**：同 mysql
- 未来 **xxx_kv**：仅 ListObjects, ExecuteRead（无 DescribeObject）

实现方式二选一（或分阶段）：

- **A) 代码内常量**：在 `templates` 或 `tool` 包中定义 `var DefaultCapabilitiesByType = map[string][]string{ "mysql": {"list_tables","describe_table","execute_read","execute_write"}, "elasticsearch": {"list_tables","describe_table","execute_read"}, ... }`，新类型时仅改此 map。
- **B) 配置扩展**：在 `data_sources[].type` 基础上，支持可选 `capabilities: [list_tables, describe_table, execute_read]`，未配置时用 DefaultCapabilitiesByType[type]。

### 3. 按请求动态注册的实现要点

- **能力矩阵**：在 `templates` 包中定义 `DefaultToolCapabilitiesByType map[string][]string`，例如：
  - `"mysql"` → `["list_tables", "describe_table", "execute_read", "execute_write"]`
  - `"elasticsearch"` → `["list_tables", "describe_table", "execute_read"]`
  未出现在 map 中的类型可回退为 mysql 能力或仅 `["list_tables", "execute_read"]`，避免空工具集。

- **DataQueryConfig** 增加 `ToolCapabilitiesByType map[string][]string`（可选）。若为 nil，则使用 DefaultToolCapabilitiesByType；FromConfig 时可用默认矩阵，也可由配置覆盖。

- **按类型注册函数**：抽成 `registerDataQueryTools(reg *tool.Registry, cfg DataQueryConfig, tools []string)`：仅将 `tools` 中列出的工具名注册到 `reg`（list_tables / describe_table / execute_read / execute_write 各根据是否在列表中决定是否 Register）。这样复用现有 ListTablesConfig、DescribeTableConfig 等，只是「按列表选择性注册」。

- **Handler 内（final 闭包）**：
  1. 解析出当前 `datasourceID`、`datasourceType`（与现有逻辑一致）。
  2. 从 `ToolCapabilitiesByType`（或默认）取 `tools := ToolCapabilitiesByType[datasourceType]`；若为空则用默认能力（如 mysql 全集或保守子集）。
  3. 若 `cfg.AllowWrite` 为 false，从 `tools` 中剔除 `execute_write`。
  4. `reg := tool.NewRegistry()`；调用 `registerDataQueryTools(reg, cfg, tools)`。
  5. `react := agent.NewReActAgent(m, mem, reg, agent.WithReActMaxSteps(cfg.MaxReActSteps))`（每请求新建 Agent，仅持有 Registry 引用，成本可接受）。
  6. 构造系统提示（已有按 datasourceType 分支），注入 system message，调用 `react.Run(ctx, &req2)` 并返回。

- **不改 Agent 包**：`ReActAgent.Run(ctx, req)` 仍只接收 request；Registry 在构造 Agent 时传入，每请求用「该请求类型对应的 Registry」构造一次 Agent 即可。

### 4. 与系统提示的配合

- 系统提示已按 **DatasourceType** 分支（mysql / elasticsearch），并描述「可用工具」；动态注册后，模型看到的工具列表与提示一致，无需再写「某工具不可用请勿调用」。
- 若某类型能力子集与默认不同（如仅 list + read），提示中可简要说明「你当前仅有 list_tables 与 execute_read」，与注册的工具集一致。

### 5. 可选：执行时校验作为兜底

- 在采用动态注册后，仍可在各工具 **Execute** 内做一次能力校验（根据 datasource_id → type → 能力列表）：若当前类型不应支持该工具，返回明确错误。用于防御「错误配置导致某类型被注册了不该有的工具」或「请求中途切换 datasource_id 等边界情况」。非必须，可按需加。

---

## 小结

| 项目 | 建议 |
|------|------|
| 工具集 | 仍统一使用 list_tables / describe_table / execute_read / execute_write 四个工具名，不按数据源拆成多套名字。 |
| 能力表达 | 用「数据源类型 → 支持的工具列表」能力矩阵（DefaultToolCapabilitiesByType + 可选配置覆盖）。 |
| 注册时机 | **按请求**：根据当前 datasource 类型从能力矩阵取工具列表，只注册这些工具到新 Registry，再用该 Registry 构造 ReActAgent 并 Run。 |
| 提示 | 继续按 DatasourceType 分支生成系统提示；模型看到的工具与提示一致。 |
| 扩展 | 新增数据源类型时，仅更新能力矩阵并实现底层 Store/Exec，无需改 Agent 或 Run 签名。 |

按上述实现后，**工具按数据源类型动态注册**，模型侧只看到当前类型支持的工具，语义清晰、体验更优雅。
