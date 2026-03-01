## 数据查询 Agent — MVP 验收清单

本清单用于验证 `DATABASE_AGENT_REQUIREMENTS.md` 中的核心能力在当前实现中是否可用。假设已经：

- 编译并安装了 `sath` 可执行文件；
- 可访问一个 MySQL 测试库（含至少 1～2 张业务表）。

### 一、配置与启动

1. **编写配置文件 `config.dev.yaml`（示例）**：

```yaml
model: "openai/gpt-4o"
max_history: 10
middlewares: ["logging", "metrics"]

data_sources:
  - id: "main"
    type: "mysql"
    host: "127.0.0.1"
    port: 3306
    user: "root"
    password: "secret"
    dbname: "demo"
    max_open_conns: 5
    max_idle_conns: 2
    conn_max_lifetime_sec: 600
    read_only: false

default_datasource_id: "main"
data_allow_write: true
```

2. **启动服务**：

```bash
sath serve --config config.dev.yaml --addr :8080
```

3. **健康检查与指标**：

- 访问 `GET /health`，返回 200 且 JSON 中 `model` 状态为 `ok`。
- 访问 `GET /metrics`，响应中包含：
  - `agent_requests_total`；
  - `dataquery_tool_calls_total`；
  - `dataquery_step_duration_seconds`。

### 二、只读能力验收（需求 2.2）

以下请求默认走 `POST /data/chat` 接口：

```bash
curl -X POST http://localhost:8080/data/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "这个库里有哪些主要的业务表？",
    "session_id": "s-mvp-1",
    "user_id": "tester"
  }'
```

**验收点**：

- 返回 200；
- 回复中能看到与实际库结构一致的表名描述（可允许语言总结而非逐表罗列）。

然后验证结构与只读查询：

1. **查看结构**：

   - 请求示例：`"帮我看看 orders 表的字段和含义。"`
   - 期望：模型解释 `orders` 表中的关键字段（如主键、金额、时间、状态）。

2. **只读查询**：

   - 请求示例：`"统计本月每个渠道的订单数和总金额，按金额倒序。"`
   - 期望：
     - 模型使用 `execute_read` 完成查询；
     - 回复为汇总结果的自然语言总结（可在指标中看到 `dataquery_tool_calls_total{tool="execute_read"}` 递增）。

### 三、写/改提议与确认（需求 2.5）

1. **提议写/改（不立即执行）**：

   - 请求示例：

   ```bash
   curl -X POST http://localhost:8080/data/chat \
     -H "Content-Type: application/json" \
     -d '{
       "message": "把昨天所有测试环境的订单状态改为已取消，可以吗？",
       "session_id": "s-mvp-2",
       "user_id": "tester"
     }'
   ```

   - 期望：
     - 模型先用只读查询评估影响范围；
     - 返回一段「变更提议」，包括：
       - 将影响多少条记录；
       - 一个确认用的 token（可在回复中显式出现）；
       - 明确说明：**当前尚未真正执行，仅为提议**。

2. **基于 token 确认执行**：

   - 用户在明确理解影响范围后，再发送类似：

   ```jsonc
   {
     "message": "好的，请按刚才的方案执行，使用 token=<上一步返回的 token>。",
     "session_id": "s-mvp-2",
     "user_id": "tester"
   }
   ```

   - 期望：
     - 工具 `execute_write` 携带 `confirm_token` 真正执行；
     - 回复中说明执行结果（受影响行数等）；
     - 在审计事件与 metrics 中可观察到：
       - `dataquery.write.proposed` 与 `dataquery.write.executed` 事件；
       - `dataquery_tool_calls_total{tool="execute_write"}` 增加；
       - `dataquery_step_duration_seconds{step="tool_execute_write"}` 有采样。

3. **只读模式限制（可选）**：

   - 将配置中的 `data_allow_write` 设为 `false`，重新启动。
   - 对同样的写/改请求：
     - 期望模型解释当前为只读模式，不会执行写/改；
     - 不产生 `execute_write` 工具调用（可通过 metrics 观察）。

### 四、E2E 行为与鲁棒性

1. **多轮对话上下文**：

   - 在同一 `session_id` 下，连续提问：
     - 「这个库有哪些表？」
     - 「刚才提到的订单表再帮我查一下最近 10 条订单。」
   - 期望：第二个问题可以基于第一轮的上下文（包括表名、含义）给出合理结果。

2. **错误与边界情况**：

   - 请求不存在表：例如「帮我查 table_does_not_exist」。
     - 期望：提示表不存在，而不是返回空结果假装成功。
   - SQL 语句语义模糊或风险过高（如「删除所有订单」）：
     - 期望：模型在提议阶段给出风险提醒，并要求用户进一步确认或缩小范围。

3. **性能与稳定性（MVP 级）**：

   - 对典型只读查询（如「统计本月订单数」），平均响应时间在合理范围内（如 < 5s，视模型延迟而定）。
   - `/metrics` 中可通过 `agent_request_duration_seconds{agent="dataquery"}` 粗略观察延迟分布。

