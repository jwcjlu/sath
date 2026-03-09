# 数据查询：按数据源类型描述符消除过度抽象与提示分支

## 1. 问题与缺陷

当前设计存在以下三点缺陷：

| 缺陷 | 表现 | 后果 |
|------|------|------|
| **1. 粗暴抽象** | 所有数据源统一抽象成 list_tables / describe_table / execute_read（+ execute_write），语义强绑在「表/列/SQL」上。 | Elasticsearch 的「索引 / mapping / Search 请求体」、未来 KV/API 的「key/读」等被强行套进「表/列」话术，语义失真。 |
| **2. 描述被固化** | 工具名与工具描述在 `tool` 包中写死（如 "List tables (or collections)..."），系统提示再按类型用 if/else 复述一遍。 | 各数据源应有的差异化表述（索引 vs 表、mapping vs 列、DSL vs SQL）被限制在一套统一名称和一套分支文案里，难以扩展且易不一致。 |
| **3. 提示词分支膨胀** | `BuildDataQuerySystemPrompt` 内对每种数据源写大段 if/else（首段、工具说明、工作流、示例）。 | 每增加一种数据源都要改多处分支，可维护性差，且提示逻辑与「类型语义」散落各处。 |

目标：**在不破坏现有架构的前提下**，让「工具集合、工具表述、系统提示」都按数据源类型**数据驱动、单一来源**，避免粗暴统一和 if/else 膨胀。

---

## 2. 设计目标

- **按类型差异化**：每种数据源类型拥有自己的「能力集合」和「表述方式」（名称/描述/提示片段），不再强行统一为 list_tables/describe_table 一套话术。
- **单一事实来源**：每种类型的工具列表、工具描述、系统提示的「类型相关部分」来自同一份「类型描述符」，而不是代码里多处 if/else。
- **易扩展**：新增一种数据源类型时，只需新增一份描述符并实现底层能力（Store/Exec），不修改 ReAct/Agent 核心逻辑，也不在 `BuildDataQuerySystemPrompt` 里加分支。
- **与现有架构兼容**：仍使用现有 Registry、ReActAgent、DataQueryConfig；描述符作为配置或注册表，在 Handler 内按类型选用。

---

## 3. 方案概述：数据源类型描述符（TypeDescriptor）

引入「**数据源类型描述符**」：每种数据源类型对应一份描述符，集中定义该类型的**工具能力**与**提示文案**，Handler 与系统提示生成**只依赖描述符**，不再按类型写 if/else。

### 3.1 描述符内容

一份类型描述符包含：

| 字段/块 | 含义 | 使用处 |
|---------|------|--------|
| **Type** | 类型标识，如 `mysql`、`elasticsearch`。 | 与 `DatasourceTypes[id]` 对应，用于查找描述符。 |
| **Tools** | 该类型支持的工具列表；每个工具可带「展示名」与「描述」覆盖。 | 注册时只注册这些工具；若提供描述覆盖，则注册到 Registry 的 Tool 使用该描述（模型可见）。 |
| **Prompt.Intro** | 类型相关的首段（如「你是一个…访问 **Elasticsearch** 数据源」）。 | 拼接到系统提示开头。 |
| **Prompt.ToolSummaries** | 可选。该类型下各工具的简短说明（用于系统提示中的「可用工具与用途」）。若为空则可从 Registry 中工具描述生成，或仅依赖模型侧从 function 描述读取。 | 若希望系统提示与工具 schema 一致，可在此写类型化表述。 |
| **Prompt.Workflow** | 该类型的推荐工作流（探索→计划→只读查询→可选写/改→解读）。 | 替换当前按类型分支的工作流段落。 |
| **Prompt.Examples** | 该类型的示例场景（列举/查看结构/只读查询/可选写改）。 | 替换当前按类型分支的示例段落。 |

**公共部分**（与类型无关）保留在 `BuildDataQuerySystemPrompt` 中一次书写：总体原则（只读/写改边界、只读模式/两阶段确认）、回答格式要求等。类型相关部分全部从描述符读取。

### 3.2 工具层：能力与表述分离

- **能力**：底层仍可复用现有实现（ListTables / DescribeTable / ExecuteRead / ExecuteWrite），通过 Exec/Store 按 datasource_id 分发到 MySQL/ES。
- **表述**：同一能力在不同类型下可有不同「展示名」和「描述」：
  - 方案 A（推荐）：**工具名保持统一**（list_tables / describe_table / execute_read / execute_write），避免 Agent/Registry 为同一能力维护多套名字；**仅允许按类型覆盖 Description（及可选 Parameters 描述）**。注册时根据当前类型从描述符取「该类型下 list_tables 的 description」传入 RegisterListTablesTool，若描述符未覆盖则用 tool 包默认描述。
  - 方案 B（更灵活、成本更高）：描述符中可为类型定义「逻辑能力 → 工具名」映射（如 elasticsearch 的 list 叫 list_indices），同一逻辑能力对应同一套参数与 Execute 实现，仅 Name/Description 按类型变化；Registry 中注册的是「带类型化名称与描述」的 Tool。新增类型时可完全自定义工具名。

**建议先采用方案 A**：实现简单、与现有 `registerDataQueryTools(reg, cfg, tools)` 兼容；仅增加「按类型传入 description 覆盖」的扩展点（例如在 Register 时接受 `ToolOption` 或 `Description string`），即可让模型侧看到「索引 / mapping / Search 请求体」等类型化描述，而不改工具名。

### 3.3 提示生成：数据驱动、无类型分支

- **BuildDataQuerySystemPrompt(cfg)** 改为：
  - 根据 `cfg.DatasourceType` 取该类型的**描述符**（若未注册则用默认描述符，如 mysql）。
  - 公共段落：总体原则、回答格式要求（仅与 AllowWrite 有关，1～2 处小分支可保留）。
  - 类型段落：`描述符.Prompt.Intro` + `描述符.Prompt.ToolSummaries`（或自动从已注册工具生成）+ `描述符.Prompt.Workflow` + `描述符.Prompt.Examples`。
  - 写/改相关：若 `cfg.AllowWrite` 为 true，追加写/改原则与示例；否则追加「只读模式」说明。这部分可仍在公共逻辑中根据 AllowWrite 写一次，或放入描述符的「可选写」片段。
- **结果**：`BuildDataQuerySystemPrompt` 内**不再出现** `if ds == "elasticsearch" { ... } else { ... }` 的多段分支；新类型只需新加描述符并注册。

### 3.4 描述符的存放与解析

- **实现方式二选一**：
  - **代码内注册表**：在 `templates` 包（或新包 `templates/dataguerytype`）中定义 `var TypeDescriptors = map[string]TypeDescriptor{}`，在 `init` 或包导入时注册 `mysql`、`elasticsearch` 等；`GetDescriptor(datasourceType string) TypeDescriptor` 未命中时返回默认（如 mysql）描述符。
  - **配置扩展**：在配置中为每种 `data_sources[].type` 提供可选的 `prompt_intro`、`workflow`、`examples` 等片段，与代码内默认描述符合并或覆盖。适合运营侧微调文案而不改代码。
- **建议**：先做代码内注册表，保证「单一事实来源」和「零 if/else」；配置覆盖可作为后续增强。

---

## 4. 与现有能力矩阵的关系

- 当前已有 **DefaultToolCapabilitiesByType**（类型 → 工具名列表）和 **registerDataQueryTools(reg, cfg, tools)**。
- **合并进描述符**：类型描述符的 **Tools** 即「该类型支持的工具列表」+ 可选的每工具描述覆盖。即：
  - `DefaultToolCapabilitiesByType` 可由「各类型描述符的 Tools 列表」推导或废弃，能力矩阵成为描述符的一部分。
  - `registerDataQueryTools(reg, cfg, tools)` 保留，但 **tools** 及「每工具描述」由描述符提供：`registerDataQueryTools(reg, cfg, descriptor.Tools)`，若 descriptor 中某工具带 Description 则注册时传入覆盖。

这样「能力矩阵」与「类型表述」统一到同一描述符，不再分散。

---

## 5. 实施要点（评审通过后执行）

1. **定义 TypeDescriptor 结构体**（如放在 `templates` 或 `templates/dataquery`）：包含 Type、Tools（含可选 Name/Description 覆盖）、Prompt（Intro、ToolSummaries、Workflow、Examples）。
2. **为 mysql、elasticsearch 各建一份描述符**：把当前 `BuildDataQuerySystemPrompt` 中与类型相关的文案迁入描述符，工具描述与现有 tool 包默认对齐，ES 使用「索引/mapping/Search」表述。
3. **tool 包扩展（方案 A）**：为 `RegisterListTablesTool` / `RegisterDescribeTableTool` / `RegisterExecuteReadTool` / `RegisterExecuteWriteTool` 增加可选参数（如 `opts *RegisterToolOptions`），其中 `Description string` 若非空则覆盖默认 Description。保持向后兼容（nil opts 或空 Description 则行为不变）。
4. **registerDataQueryTools**：改为接受 `descriptor`（或 tools + 每工具描述 map），注册时对每个工具若描述符中有描述则传入 tool 包。
5. **BuildDataQuerySystemPrompt**：改为按 `cfg.DatasourceType` 取描述符；公共段落保留；类型段落全部从 `descriptor.Prompt` 拼接；删除所有按类型的 if/else 大块。
6. **Handler**：final 闭包内根据 datasourceType 取描述符 → 从描述符取 Tools 列表（并应用 AllowWrite 过滤）→ `registerDataQueryTools(reg, cfg, descriptor)` → 构造系统提示时使用同一描述符。无需再维护单独的 DefaultToolCapabilitiesByType（或仅作默认描述符的 Tools 来源）。

---

## 6. 小结

| 项目 | 当前问题 | 改进后 |
|------|----------|--------|
| 工具抽象 | 所有类型统一 list_tables/describe_table/execute_read，语义偏 SQL | 工具名可保持统一，**描述按类型覆盖**，ES 用索引/mapping/Search 等表述；扩展可选方案 B 支持类型自有工具名。 |
| 描述来源 | 工具描述写死在 tool 包，提示中再 if/else 复述 | **类型描述符** 提供该类型下工具描述（及可选提示中的工具小结）；注册时注入，提示从描述符拼，单一来源。 |
| 提示分支 | BuildDataQuerySystemPrompt 内大量 if/else | **按类型取描述符，提示由描述符片段 + 公共段落组成**，无类型分支；新类型只新增描述符并注册。 |
| 能力矩阵 | DefaultToolCapabilitiesByType 与提示分离 | **合并进类型描述符**，能力列表与表述一处定义，Handler 与注册逻辑只读描述符。 |

**预期效果**：新数据源类型只需新增一份类型描述符并实现底层 Store/Exec，即可得到一致的工具集、类型化描述和系统提示，无需改 ReAct、Agent 或到处加 if/else。方案评审通过后再按上述要点实施。
