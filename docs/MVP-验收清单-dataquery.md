# 数据查询 Agent MVP 验收清单

与《DATABASE_AGENT-开发任务拆解-Cursor-Agent.md》T-13 及需求 2.2、2.5 对应，用于验收数据查询 Agent 的端到端能力。

---

## 1. 环境与配置

- [ ] 配置文件包含 `data_sources`（至少一个 MySQL 或 Elasticsearch）与 `default_datasource_id`。
- [ ] 可选：`data_allow_write: true` 时开放写/改及两阶段确认；`false` 时仅只读。
- [ ] 未配置任何数据源时，POST /data/chat 返回 503（data query handler not configured）。

## 2. 列举数据对象（对应需求 2.2）

- [ ] 用户发送「有哪些表」或「列举索引」类消息。
- [ ] Agent 调用 list_tables（或等价能力），回复中包含表/索引列表或说明。

## 3. 查看结构/映射（对应需求 2.2）

- [ ] 用户发送「看看某表/某索引的字段」类消息。
- [ ] Agent 调用 describe_table，回复中包含列/mapping 等结构说明。

## 4. 只读查询（对应需求 2.2、2.5）

- [ ] 用户发送只读类问题（如「查询某表 limit 2」「统计某指标」）。
- [ ] Agent 调用 execute_read，返回结果集或汇总；不触发写操作。
- [ ] 若用户尝试写/改且当前为只读模式，Agent 明确说明仅支持查询并给出替代建议。

## 5. 写/改提议与确认（对应需求 2.5，仅当 data_allow_write 为 true）

- [ ] 用户明确要求写/改（如 UPDATE/DELETE/INSERT）。
- [ ] Agent 先通过 execute_write **提议**（不传 confirm_token），返回待确认信息及 token。
- [ ] 用户基于该 token 明确确认后，Agent 再次调用 execute_write **携带 confirm_token** 执行。
- [ ] 未携带有效 token 或 token 过期/已使用时，写/改被拒绝。

## 6. API 与可观测

- [ ] POST /data/chat 请求体含 `message`；可选 `session_id`、`user_id`、`datasource_id`；响应 JSON 含 `reply`。
- [ ] 若涉及写/改待确认，响应可含 `confirm_required`、`confirm_request`（含 token）。
- [ ] GET /metrics 可看到 dataquery 相关指标（如 `dataquery_tool_calls_total`、`dataquery_step_duration_seconds`）。

## 7. 验收方式

- **单测**：`go test ./templates/... ./tool/... ./executor/... ./metadata/... ./datasource/... ./obs/...` 通过。
- **手动/本地**：使用 `examples/database_agent` 配置与 `sath serve -config <path>` 启动服务，按上列步骤用 curl 或前端调用 POST /data/chat 逐项验证。
- **Elasticsearch**：配置 `type: elasticsearch` 数据源后，上述「列举/结构/只读」步骤使用索引与 Search 请求体语义一致。
