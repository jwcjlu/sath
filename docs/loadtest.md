# 压测说明（NF1 / C.3.2）

本文档说明如何对 `sath serve` 提供的 HTTP 接口做基本压测，用于验证 100 QPS 等 NF1 指标。

## 前置条件

- 已启动服务：`sath serve -a :8080`（或使用 `-c config.yaml`）
- 安装压测工具之一：**hey** 或 **wrk**

## 安装 hey（推荐）

```bash
go install github.com/rakyll/hey@latest
```

## 健康检查端点

```bash
# 1000 请求，50 并发
hey -n 1000 -c 50 http://localhost:8080/health
```

关注输出中的 **Requests/sec**、**Latency distribution**（如 P99）。

## /chat 端点

`POST /chat` 需要 JSON 体 `{"message":"..."}`。使用 hey 的 `-m` 和 `-D`：

```bash
echo '{"message":"hi"}' > /tmp/chat.json
hey -n 500 -c 20 -m POST -D /tmp/chat.json -H "Content-Type: application/json" http://localhost:8080/chat
```

注意：实际 QPS 会受模型 API 延迟影响；若需逼近 100 QPS，可配合 mock 或限流中间件调整并发数。

## 目标参考（NF1）

- 在无下游模型阻塞的前提下，**/health** 等轻量端点建议 P99 延迟 &lt; 100ms，QPS 可远超 100。
- **/chat** 的 QPS 主要受模型 API 与超时限制，可根据业务设定目标（如 P99 &lt; 5s、可接受 QPS ≥ 10）。

## wrk 示例

```bash
# 需 lua 脚本发送 POST body，或仅压 health
wrk -t4 -c50 -d10s http://localhost:8080/health
```

更复杂的 /chat 压测可编写 wrk 的 lua 脚本或继续使用 hey。
