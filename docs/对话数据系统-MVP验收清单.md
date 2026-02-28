# 对话式数据查询与修改系统 — MVP 验收清单

与 `docs/对话数据系统-开发任务拆解-Cursor-Agent.md` 中 T-17 对应。

## 验收项

1. **配置 MySQL 数据源并拉取元数据**
   - 在 YAML 配置中增加 `data_sources` 与 `default_datasource_id`（或使用环境变量 `DEFAULT_DATASOURCE_ID`）。
   - 启动 `sath serve -c config.yaml` 后，数据源注册成功，元数据在首次请求时可按需刷新。

2. **通过 POST /data/chat 发送「有哪些表」并返回表列表**
   - 请求体：`{"message": "有哪些表", "session_id": "test-session"}`。
   - 响应 200，`reply` 中包含当前数据源下的表列表（或「当前无表信息」）。

3. **发送「查询 xxx 表 limit 2」并返回 2 行**
   - 请求体：`{"message": "查询 users 表 limit 2", "session_id": "test-session"}`。
   - 响应 200，`reply` 或 `data.rows` 中为最多 2 行数据。

4. **发送修改类请求并返回待确认，确认后执行成功**
   - 先发送：`{"message": "把 users 表里 id=1 的名字改成 test"}`（或等价自然语言）。
   - 响应 200，`confirm_required` 为 true，`reply` 中提示确认。
   - 再发送：`{"message": "确认", "session_id": "同上", "confirm_token": "任意非空"}`（或按实际约定传 token）。
   - 响应 200，`reply` 中为「影响行数：1」或执行结果。

## 运行端到端测试

```bash
go test ./agent/... -v -run DataQuery
go test ./templates/... -v
```

无真实 MySQL 时，数据源相关集成测试会跳过或使用 mock。
