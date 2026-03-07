---
name: k8s-ops
description: Kubernetes 集群运维、排障与部署；通过 mcp-k8s 提供的工具进行配置查看、事件/资源列表、Pod 与节点监控、Helm 与资源增删改等操作。
tags:
  - k8s
  - kubernetes
  - devops
  - cluster
scope: chat
allowed_tools:
  - load_skill
  - read_skill_file
mcp_servers:
  - mcp-k8s
mcp_tools:
  - configuration_view
  - events_list
  - helm_install
  - helm_list
  - helm_uninstall
  - namespaces_list
  - nodes_log
  - nodes_stats_summary
  - nodes_top
  - pods_delete
  - pods_exec
  - pods_get
  - pods_list
  - pods_list_in_namespace
  - pods_log
  - pods_run
  - pods_top
  - resources_create_or_update
  - resources_delete
  - resources_get
  - resources_list
  - resources_scale
---
# Kubernetes 运维 Skill（k8s-ops）

## 何时使用本 Skill

- 用户需要与 Kubernetes 集群交互：查看配置与事件、列 Pod/命名空间/节点、查日志、扩缩容、Helm 安装/卸载、资源增删改等。
- 本 Skill 依赖 **mcp-k8s**。加载本 Skill 后，若配置中已提供 id 为 `mcp-k8s` 的 MCP 服务，下列工具会注册到当前上下文，可直接调用与集群交互。

## mcp-k8s 可用工具一览

| 工具名 | 用途简述 |
|--------|----------|
| **configuration_view** | 查看集群/上下文配置 |
| **events_list** | 列出事件（可按命名空间等过滤） |
| **namespaces_list** | 列出命名空间 |
| **nodes_log** | 节点日志 |
| **nodes_stats_summary** | 节点资源统计摘要 |
| **nodes_top** | 节点资源占用（类似 kubectl top nodes） |
| **pods_list** | 列出 Pod（可跨命名空间） |
| **pods_list_in_namespace** | 列出指定命名空间内的 Pod |
| **pods_get** | 获取单个 Pod 详情 |
| **pods_log** | 获取 Pod 日志（可指定容器） |
| **pods_top** | Pod 资源占用（类似 kubectl top pods） |
| **pods_run** | 在集群中运行 Pod（如临时调试） |
| **pods_exec** | 在 Pod 内执行命令 |
| **pods_delete** | 删除 Pod |
| **resources_list** | 列出某类资源（如 deployment、service） |
| **resources_get** | 获取单个资源详情 |
| **resources_create_or_update** | 创建或更新资源（apply） |
| **resources_delete** | 删除资源 |
| **resources_scale** | 扩缩容（如 deployment replicas） |
| **helm_install** | Helm 安装 Release |
| **helm_list** | Helm 列出 Release |
| **helm_uninstall** | Helm 卸载 Release |

## 工作流建议

1. **确认上下文**：明确命名空间、资源类型（Pod、Deployment、Service 等），必要时先调用 `namespaces_list` 或 `configuration_view`。
2. **只读优先**：先使用 `pods_list` / `pods_list_in_namespace`、`pods_get`、`events_list`、`pods_log`、`nodes_top` / `pods_top` 等获取状态，再执行写操作。
3. **写操作前确认**：对 `pods_delete`、`resources_delete`、`resources_scale`、`resources_create_or_update`、`helm_install` / `helm_uninstall` 等，说明影响范围并建议在测试环境先验证。
4. **安全与权限**：提醒用户注意 RBAC、生产环境变更窗口与回滚方案。

## 典型场景与对应工具

- **查看命名空间与 Pod**：`namespaces_list` → `pods_list_in_namespace` 或 `pods_list`。
- **查看某 Pod 详情与事件**：`pods_get`，配合 `events_list`（按命名空间/资源过滤）。
- **排障（日志与资源）**：`pods_log`、`nodes_log`；`pods_top`、`nodes_top`、`nodes_stats_summary`。
- **扩缩容**：`resources_scale`（如 Deployment）。
- **创建/更新/删除资源**：`resources_create_or_update`、`resources_delete`；列表与详情用 `resources_list`、`resources_get`。
- **临时运行/调试**：`pods_run`、`pods_exec`。
- **Helm 管理**：`helm_list` 查看；`helm_install` 安装；`helm_uninstall` 卸载。

## 可选文档

- 可通过 `read_skill_file("k8s-ops", "docs/cheatsheet.md")` 获取命令速查表（若存在）。

## 注意事项

- 写操作（删除、扩缩容、创建/更新、Helm 安装/卸载）需谨慎，涉及生产时强调测试环境验证与变更审批。
- 具体参数以各 MCP 工具定义为准（如命名空间、资源名、容器名等）。
