## Gateway 需求文档（反向整理）

### 一、问题域与目标

#### 1.1 项目背景

OpenClaw 是一个本地优先的多通道 AI 助手系统，支持 WhatsApp、Telegram、Slack、Discord、Signal、iMessage、WebChat 等多种消息渠道，以及 macOS/iOS/Android 等节点设备。系统需要一个统一的控制平面来：

- 统一管理渠道连接与状态；
- 统一调度智能体（agents）与工具；
- 对外提供稳定、类型化的控制与集成功能。

这个控制平面即 **Gateway**。

#### 1.2 Gateway 的角色

Gateway = 单实例控制平面 + 传输层网关：

- **唯一网关进程**：一台主机上只允许有一个 Gateway 实例控制所有渠道连接（特别是 WhatsApp Baileys 会话）。
- **全局真相源**：对会话、配置、渠道状态、节点、定时任务等拥有最终权威数据。
- **传输中枢**：对所有控制面客户端（CLI、macOS App、Web UI、自动化）和节点（macOS/iOS/Android/headless）提供统一 WebSocket 协议。

#### 1.3 目标

- **统一协议**：为所有客户端/节点提供同一套 WebSocket 协议，覆盖控制面、节点能力调用、会话管理等。
- **多角色安全接入**：支持 operator / node 角色，提供 scopes + 设备配对 + token 安全模型。
- **高内聚控制面**：
  - 单端口同时承载 HTTP（Control UI / Canvas / OpenAI 端点）和 WebSocket。
  - 集中管理 cron、exec approvals、secrets、skills、models 等功能。
- **可扩展**：渠道和插件可以扩展 Gateway 方法与事件，而不破坏核心协议。

#### 1.4 非目标 / 约束

- Gateway 本身不负责 LLM 的业务逻辑（由 Pi Agent 运行时承担），而负责编排与安全。
- 不负责客户端 UI 的呈现逻辑，只提供 HTTP 静态资源与 WS API。
- 不支持同一主机上多 Gateway 共享同一渠道会话（特别是 WhatsApp）。

---

### 二、高层架构

#### 2.1 运行时拓扑

- 进程模型：单 Node.js 进程（Node ≥ 22），模块化架构（ESM）。
- 监听端口：一个端口上同时承载：
  - HTTP：Control UI、Canvas host、A2UI、OpenAI/OpenResponses 兼容端点；
  - WebSocket：Gateway 协议。

#### 2.2 关键参与者

- Gateway 进程（`src/gateway/*`）
- 控制面客户端（role=operator）：
  - CLI（`openclaw gateway`、`openclaw agent`、`openclaw sessions` 等）
  - macOS App、Web 控制台、自动化脚本等
- 节点（role=node）：
  - macOS/iOS/Android/headless 设备，提供 camera/screen/canvas/location/voice 等能力
- Agent 运行时：
  - 嵌入式 Pi Agent（`src/agents/*`），由 Gateway 通过 RPC 驱动
- 渠道与插件：
  - 所有消息渠道（WhatsApp/Telegram/Slack/Discord/...）与 `extensions/*` 插件

---

### 三、协议与传输设计

#### 3.1 传输层

- 介质：WebSocket over TCP，文本帧，JSON 负载。
- 首帧约束：
  - 第一个非控制帧必须是 `connect` 请求；
  - 若首帧不是 JSON 或不是 `method: "connect"`，立即 hard close。

#### 3.2 握手流程（需求）

1. 客户端连接 WebSocket 端点。
2. Gateway 向客户端发送 `type:"event", event:"connect.challenge"`，payload 至少包含：
   - `nonce`: 服务器生成的随机字符串；
   - `ts`: 服务器当前时间戳。
3. 客户端在本地构造 `connect` 请求：
   - 填充协议版本：`minProtocol`, `maxProtocol`；
   - 声明 client 信息：`id`, `version`, `platform`, `mode`；
   - 设置 `role`：`operator` 或 `node`；
   - 设置 `scopes`：授权级别（operator.*）；
   - 对于 Node：声明 `caps`, `commands`, `permissions`；
   - 提供 `auth.token`（如果 Gateway 配置了 token）；
   - 构造 `device` 对象：`id`, `publicKey`, `signature`, `signedAt`, `nonce`；
     - `signature` 必须覆盖包含 nonce 在内的标准 payload（v2/v3 格式）。
4. Gateway 对上述字段做校验：
   - 协议版本是否兼容；
   - token 是否匹配；
   - 设备身份与签名是否有效且未过期；
   - 若为新设备，是否需要配对审批。
5. 校验通过后，Gateway 以 `hello-ok` 响应：
   - 指定最终 `protocol` 版本；
   - 提供 `policy`（如 `tickIntervalMs`）；
   - 如需，下发 `auth.deviceToken`（与 role + scopes 绑定）。

#### 3.3 帧类型（需求）

请求帧：

```json5
{ "type": "req", "id": "<string>", "method": "<string>", "params": { ... } }
```

响应帧：

```json5
// 成功
{ "type": "res", "id": "<string>", "ok": true, "payload": { ... } }

// 失败
{ "type": "res", "id": "<string>", "ok": false, "error": {
  "code": "<string>",
  "message": "<string>",
  "details"?: { ... 诊断信息 ... }
} }
```

事件帧：

```json5
{ "type": "event", "event": "<string>", "payload": { ... }, "seq"?: <number>, "stateVersion"?: { ... } }
```

幂等等价性需求：

- 对所有具有副作用的请求（例如 `send`, `agent`, `chat.send`）必须要求携带幂等键；
- Gateway 需要维持短时幂等缓存（去重窗口），确保重试不会产生多次实际副作用。

---

### 四、角色、Scope 与配对模型

#### 4.1 角色（role）

- `operator`：控制面客户端（CLI、UI、自动化）。
- `node`：能力提供者（camera/canvas/screen/voice 等）。

#### 4.2 Scopes（对于 operator）

需求示例：

- `operator.read`：只读操作（如 `health`、`status`、`sessions.list`）。
- `operator.write`：有副作用的操作（如 `send`、`agent`）。
- `operator.admin`：变更全局配置（如 `/config set`）。
- `operator.approvals`：处理 exec approvals。
- `operator.pairing`：设备/节点配对与 token 管理。

#### 4.3 节点能力声明（node caps/commands/permissions）

- `caps`：高层能力标识，如 `["camera", "canvas", "screen", "location", "voice"]`。
- `commands`：节点可接受的 `node.invoke` 命令名白名单，如 `"camera.snap"`, `"canvas.navigate"`。
- `permissions`：更细粒度的授权位，如 `"screen.record": false`, `"camera.capture": true`。
- Gateway 仅将其视为声明，仍需在服务端按配置/策略做最终校验。

#### 4.4 设备身份与配对

需求：

- 所有 WS 客户端（包括 operator 与 node）必须在 `connect.params.device` 中提供：
  - 稳定的设备 ID（建议由公钥指纹派生）；
  - 公钥与基于服务器 nonce 的签名；
- Gateway 需要维护一个设备表：
  - 每条记录绑定设备 id、公钥、当前 token、允许的 roles + scopes。
- 新设备接入时：
  - 非本地连接必须走配对流程：发 pairing 请求 → operator 审批 → 发 device token。
  - 本地（loopback 或 gateway 主机 tailnet 地址）可按配置选择自动批准。
- 设备 token 支持：
  - 旋转：`device.token.rotate`；
  - 吊销：`device.token.revoke`。

---

### 五、网关 API 面（方法与事件）

#### 5.1 方法总表（抽象需求）

Gateway 至少需要支持以下方法类别：

- 健康与状态：
  - `health`, `status`, `channels.status`, `channels.logout`
  - `usage.status`, `usage.cost`
  - `last-heartbeat`, `set-heartbeats`
- 配置管理：
  - `config.get`, `config.set`, `config.apply`, `config.patch`
  - `config.schema`, `config.schema.lookup`
- 会话管理：
  - `sessions.list`, `sessions.preview`, `sessions.patch`, `sessions.reset`, `sessions.delete`, `sessions.compact`
- Agent 与消息：
  - `send`：从某个渠道/账号向某个 peer 发送消息；
  - `agent`：触发一次智能体运行；
  - `agent.identity.get`, `agent.wait`
  - `agents.list`, `agents.create`, `agents.update`, `agents.delete`
  - `agents.files.list/get/set`：AGENTS/SOUL/TOOLS 等工作区文件访问。
- 技能、工具、模型：
  - `skills.status`, `skills.bins`, `skills.install`, `skills.update`
  - `tools.catalog`
  - `models.list`
- 节点与设备：
  - `node.pair.request`, `node.pair.list`, `node.pair.approve`, `node.pair.reject`, `node.pair.verify`
  - `node.rename`, `node.list`, `node.describe`
  - `node.invoke`, `node.invoke.result`, `node.event`
  - `device.pair.list`, `device.pair.approve`, `device.pair.reject`, `device.pair.remove`
  - `device.token.rotate`, `device.token.revoke`
- 定时任务（Cron）：
  - `cron.list`, `cron.status`, `cron.add`, `cron.update`, `cron.remove`, `cron.run`, `cron.runs`
- 执行审批（Exec approvals）：
  - `exec.approvals.get`, `exec.approvals.set`, `exec.approvals.node.get`, `exec.approvals.node.set`
  - `exec.approval.request`, `exec.approval.waitDecision`, `exec.approval.resolve`
- 向导与语音：
  - `wizard.start`, `wizard.next`, `wizard.cancel`, `wizard.status`
  - `talk.config`, `talk.mode`
  - `voicewake.get`, `voicewake.set`
- Secrets 与系统：
  - `secrets.reload`, `secrets.resolve`
  - `system-presence`, `system-event`
- 浏览器与 WebChat：
  - `browser.request`
  - `chat.history`, `chat.send`, `chat.abort`

扩展机制需求：

- 渠道插件可以在自身定义 `gatewayMethods` 列表；
- Gateway 在运行时通过 `listChannelPlugins()` 收集所有插件方法，并与基础方法集合并（去重）得到完整方法集。

#### 5.2 事件总表（需求）

最少需要支持：

- `connect.challenge`（前握手事件）
- `agent`（智能体流式结果）
- `chat`（聊天事件，主要给 WebChat/应用）
- `presence`（系统与节点在线状态）
- `tick`（周期性心跳）
- `talk.mode`（语音模式变更）
- `shutdown`（Gateway 关闭通知）
- `health`（健康状态快照）
- `heartbeat`（心跳调度事件）
- `cron`（定时任务事件）
- `node.pair.requested`, `node.pair.resolved`（节点配对）

---

### 六、内部组件与流程（详细设计层面的需求）

#### 6.1 服务启动（startGatewayServer）

需求：

1. 设置运行时环境变量（如 `OPENCLAW_GATEWAY_PORT`）。
2. 加载配置（含 Nix mode、迁移旧版 config 等）。
3. 应用启动时 auth/tailscale 的覆盖项（合并配置与 CLI 传参）。
4. 初始化：
   - 日志子系统；
   - Gateway 运行时状态；
   - 会话键解析器；
   - 节点注册表、子 agent 注册表、技能远程注册表；
   - cron 服务；
   - channel manager（渠道连接/重连与健康监控）；
   - exec approvals 管理器；
   - secrets runtime snapshot；
   - 浏览器控制服务（如启用）；
   - Tailscale 暴露；
   - Canvas host 与 Control UI 静态资源路径；
5. 启动 HTTP + WebSocket 服务：
   - HTTP 负责静态资源 + REST 端点（OpenAI 兼容等）；
   - WebSocket 通过连接处理器完成：
     - `connect.challenge` 与 `connect` 握手；
     - 构建每个连接的上下文（角色、scopes、device 信息）；
     - 将请求路由到对应的处理器；
     - 通过 `broadcast` 向客户端推送事件。
6. 启动维护任务：
   - 健康状态缓存、心跳调度；
   - Gateway 自检；
   - 更新检查；
   - 媒体清理（按 configurable TTL）。

#### 6.2 会话与路由

- 会话键：
  - 基于代理 id、渠道、账号、peer、群/团队信息生成；
  - direct chat 默认聚合到 `agent:<agentId>:main`，可由 `session.dmScope` 控制（`main`/`per-peer`/`per-channel-peer`/`per-account-channel-peer`）。
- 路由：
  - 解析输入：`channel`, `accountId`, `peer`, `parentPeer`, `guildId`, `teamId`, `memberRoleIds`；
  - 按优先级匹配绑定（peer/guild+roles/guild/team/account/channel/default）得到 `agentId` + sessionKey；
  - 生成 `mainSessionKey` 与 `lastRoutePolicy`，用于决定更新最近路由信息的会话。

#### 6.3 Auto-reply 与派发

- 为同一会话内消息维护一个或多个 reply dispatcher；
- Gateway 在关闭或重启前应等待所有 dispatcher 变为空闲（`waitForIdle`），确保没有在途回复丢失；
- 统计全局 pending 数量，为 health/status 提供指标。

#### 6.4 节点与设备管理

- 节点配对生命周期：
  - request → list → approve/reject → verify；
  - 对外发 `node.pair.requested` / `node.pair.resolved` 事件。
- 设备表管理：
  - list + approve/reject/remove；
  - device token 的 rotate/revoke；
- 为节点提供 `node.invoke` / `node.event` 与结果回送；对每次 invoke 处理执行审批等策略。

---

### 七、错误处理与诊断（需求）

- 所有错误响应必须提供稳定的 `error.code` 与可读的 `message`，详尽诊断信息放在 `details`。
- 对于设备认证错误，需要给出稳定的 `DEVICE_AUTH_*` code 与 `details.reason`，便于客户端迁移。
- 应提供：
  - `health` / `status` / `channels.status` 等可编程诊断接口；
  - `logs.tail` 等日志流访问；
  - `doctor.*` 类 API 用于自动诊断常见问题。

---

### 八、配置与部署（需求）

- Gateway 配置集中在 `~/.openclaw/openclaw.json` 或 Nix mode 配置：
  - `gateway.bind`（loopback/lan/tailnet/auto）；
  - `gateway.http`（Control UI / OpenAI-compatible / OpenResponses 开关）；
  - `gateway.auth`（token、TLS、远程访问策略）；
  - `gateway.tailscale`（暴露与发现）；
  - 以及 `session.*`、`agents.defaults.*`、`channels.*` 等。
- 启动方式：
  - CLI：`openclaw gateway`、`openclaw gateway --port 18789`；
  - 由系统服务托管：launchd/systemd/schtasks。

---

### 九、非功能性需求

- 单机可靠性：
  - Gateway 需配合系统服务实现自动重启；
  - 支持软重启策略与优雅关闭（等待 pending replies）。
- 安全：
  - 强制 `connect.challenge` 与设备签名；
  - 可配置强制 token（`OPENCLAW_GATEWAY_TOKEN`）；
  - 角色 + scope + exec approvals 分层控制风险操作；
  - DM pairing / allowlist 配置由 Gateway 校验。
- 扩展性：
  - 渠道插件可以按统一接口扩展方法/事件；
  - 插件可注册 CLI 子命令，但 Gateway 方法面需保持 schema 受控（TypeBox）。
- 可观测性：
  - 结构化日志与子系统 logger；
  - 健康状态缓存、presence、usage metrics；
  - `logs.tail` 与 `health`/`status` API。

