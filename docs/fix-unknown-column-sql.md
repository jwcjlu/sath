# 修复建议：未知列名导致 SQL 报错（Unknown column in 'field list'）

## 根因补充：为何提示已出现但 Agent 仍未自我修复

现象：日志中已出现「请先对该表/索引调用 describe_table 获取正确结构后再重试 execute_read」，但请求仍以 **error** 结束，且用户最终看到的是该错误信息而非模型在下一轮调用 describe_table 后的结果。

**根因**：在 `model/openai_tools.go` 的 `ChatWithTools` 中，当工具执行返回错误时，实现为**直接将该错误作为函数返回值**（`return nil, fmt.Errorf("execute tool %s: %w", ...)`），而不是把错误内容作为**工具观察（observation）** 交给模型。因此：

1. ReAct 循环在第一步执行 `execute_read` 失败后，收到的是 `ChatWithTools` 的 err，立即退出并把该错误返回给调用方。
2. 模型**从未看到**「execute_read 失败 + describe_table 提示」这段观察，因此无法在下一轮先调用 describe_table 再重试。

**结论**：工具执行失败时应将错误**作为本次工具调用的观察结果**返回（即返回成功的 `Generation`，其 `Text` 为错误信息），让 ReAct 继续循环、模型根据观察决定下一步（如先 describe_table 再 execute_read），而不是让整次请求因工具错误而直接失败。

---

## 问题现象（原始）

用户问「查询每个用户的订单总额」时，Agent 直接生成 SQL 并调用 `execute_read`，使用了表中不存在的列名（如 `order_amount`），导致：

```text
Error 1054 (42S22): Unknown column 'order_amount' in 'field list'
```

## 根因

- 现有能力：已有 `list_tables`、`describe_table`、`execute_read`，且 prompt 中推荐「先 list_tables，再对关键表 describe_table」。
- 实际行为：模型未严格遵循该流程，在**未调用 describe_table 获取真实列名**的情况下就编写 SQL，凭臆测使用列名（如 `order_amount`），导致 1054 错误。

因此问题在于：**约束不够硬**（仅“推荐”而非“必须”）+ **报错后缺少明确引导**（未告诉模型应先去 describe_table）。

---

## 修复方案（评审用）

### 方案一：强化系统提示（必做，推荐）

**思路**：把「写 SQL 前必须先掌握表结构」从“推荐”升级为**硬性规则**，并禁止臆造列名。

**修改文件**：`templates/dataquery.go` — `BuildDataQuerySystemPrompt`

**具体改动**：

1. 在 **【总体原则】** 或 **【可用工具与用途】** 后增加一条显式规则，例如：

   ```text
   - **写 SQL 前必须已知表结构**：在调用 execute_read 或 execute_write 编写涉及某张表的 SQL 之前，你必须已对该表调用过 describe_table，并**仅使用 describe_table 返回的列名**。禁止臆造列名（如 order_amount、total_price、user_name 等），否则会报 Unknown column 错误。
   ```

2. 在 **【推荐工作流】** 第 1 条「探索阶段」中，把“再根据需要对关键表使用 describe_table”改为更强表述，例如：

   ```text
   探索阶段：当用户第一次接入或你不了解库结构时，先使用 list_tables 获取有哪些表；在编写任何涉及某张表的 SELECT/写操作之前，**必须**先对该表调用 describe_table，并以返回的列名、类型为准编写 SQL。
   ```

3. 在 **【示例场景】** 的「只读查询」示例中补一句，例如：

   ```text
   只读查询：……在思考中说明将使用 orders 表、按渠道与月份分组统计；**若尚未获取过 orders 的列信息，先调用 describe_table(table_name=\"orders\")**，再通过 execute_read 执行 SELECT……
   ```

**预期效果**：模型在写 SQL 前更倾向于先调用 `describe_table`，减少 Unknown column。

---

### 方案二：execute_read 报错时引导重试（已实现，并做泛化）

**思路**：当 `execute_read` 因「表/字段结构相关」错误失败时，在返回给模型的错误信息中附带「先 describe_table 再重试」的指引。为支持多种数据源（MySQL、Elasticsearch、PostgreSQL 等），采用**约定错误类型**而非写死数据库错误码。

**泛化设计**：

1. **executor 包**（`executor/executor.go`）：
   - 定义 `SchemaRelatedError` 类型（包装原始 error，实现 `Error()` 与 `Unwrap()`）。
   - 提供 `IsSchemaRelated(err error) bool`（通过 `errors.As` 判断链中是否包含 `SchemaRelatedError`）。
   - 语义：表示「与表/列/字段/mapping 等结构相关的执行错误」，与具体数据库无关。

2. **各数据源实现**（如 `executor/mysql.go`）：
   - 在返回错误前，根据**本数据源**的报错特征判断是否为「结构相关」：
     - MySQL：错误信息包含 `Unknown column`、`1054`、`42S22` 等时，将错误包装为 `&SchemaRelatedError{Err: wrapped}` 再返回。
     - Elasticsearch：可在检测到 "No mapping found"、"unknown field" 等时同样包装为 `SchemaRelatedError`。
     - PostgreSQL：可在 "column \"xxx\" does not exist"、`42703` 等时包装。
   - 工具层不关心具体数据库，只认「是否 SchemaRelated」。

3. **工具层**（`tool/execute_read.go`）：
   - 在 `cfg.Exec.Execute` 返回 `err != nil` 时，若 `executor.IsSchemaRelated(err)` 为 true，则追加提示：`请先对该表/索引调用 describe_table 获取正确结构后再重试 execute_read。`
   - 不再使用任何数据库特定的字符串（如 "Unknown column"、"1054"）。

**效果**：MySQL/ES/PG 等均可复用同一套「结构错误 → describe_table 提示」逻辑；新增数据源只需在对应 executor 里在合适错误处包装 `SchemaRelatedError` 即可。

---

### 方案三：不在本阶段做的选项（仅说明）

- **自动注入表结构**：在每次请求时自动对“可能用到的表”调用 describe_table 并塞进 system prompt。需要先解析用户意图得到表名，实现复杂，且可能注入过多 token，暂不推荐。
- **执行前校验列名**：在 executor 层执行 SQL 前解析 SQL 并校验列名是否在 metadata 中。实现成本高、且需考虑多种 SQL 写法，建议作为后续增强。

---

## 建议实施顺序

1. **先做方案一**（仅改 prompt）：改动小、无兼容性风险，能显著降低“不知道表结构就写 SQL”的概率。
2. **再做方案二**（execute_read 错误增强）：在仍出现 1054 时，给模型一次“先 describe_table 再重试”的明确指引，提高单次对话内自我修复率。

---

## 验收预期

- 用户问「查询每个用户的订单总额」时，Agent 应至少先对相关表（如 orders 或用户/订单表）调用 `describe_table`，再根据返回列名生成 SELECT，避免 Unknown column。
- 若因模型仍偶尔跳过 describe_table 而再次报 1054，错误信息中应包含“请先对该表调用 describe_table…”的提示，且下一轮中模型应能去 describe_table 并重试。

---

请评审上述方案，通过后再按方案一、方案二顺序修改代码。
