# Skills 能力需求文档（与 MCP 协同的知识层设计）

## 1. 背景与目标

### 1.1 背景

- 项目当前已经有成熟的 **ReAct + Tools** 架构：
  - `tool` 包提供统一的 `Tool` 抽象与 `Registry`；
  - `agent.ReActAgent` 实现「思考 → 工具 → 观察」循环；
  - `templates` 包封装了 `NewDataQueryHandlerFromConfig`、`NewChatAgentHandlerFromConfig` 等 Handler。
- 项目也在使用或规划使用 **MCP（Model Context Protocol）**，通过 MCP 将 Agent 与外部系统连接（数据库、服务、MCP server 等），解决「**连接性（Connectivity）**」问题（参考 [Model Context Protocol 文档](https://modelcontextprotocol.io/)）。
- Anthropic 提出的 **Agent Skills**（参考：
  - [Agent Skills 文档](https://docs.anthropic.com/en/docs/agent-skills)
  - [anthropics/skills 仓库](https://github.com/anthropics/skills)
  - [Improving frontend design through Skills](https://www.claude.com/blog/improving-frontend-design-through-skills)
  - [Agent Skills 与 MCP：智能体能力扩展的两种范式](https://github.com/jwcjlu/hello-agents/blob/main/Extra-Chapter/Extra05-AgentSkills%E8%A7%A3%E8%AF%BB.md)
  ）提供了一种将**领域知识 + 工作流 SOP** 封装为标准资产的方式。

总结自这些资料：

- **MCP** 侧重于「**能连上**」：提供标准化的工具/API 接口。
- **Skills** 侧重于「**会用好**」：告诉模型在特定任务/领域下应如何分解问题、如何使用工具、有哪些最佳实践。

### 1.2 目标

在现有架构基础上，引入 **Skills 子系统**，满足：

- 把领域知识、业务工作流、最佳实践**标准化封装**为可复用、可分享的 Skill；
- 通过 **渐进式披露（Progressive Disclosure）** 机制按需加载 Skill 内容，避免 system prompt 永久膨胀；
- 与现有 ReAct / Tools / MCP 架构**自然衔接**：
  - Skills 不替代 tools、也不替代 MCP；
  - Skills 通过「文件/资源」形式向 Agent 提供“操作手册”，指导其更好地调用现有工具（包括 MCP 工具）。

---

## 2. 核心概念与术语

### 2.1 Skill

**Skill**：一份可复用的、结构化的领域知识与流程说明，通常对应一个**特定任务类型或领域**，例如：

- `frontend-design`：指导如何生成具有品牌感和设计感的前端；
- `mysql-employees-analysis`：指导如何分析 MySQL employees 示例库；
- `code-review-workflow`：指导如何进行标准化的代码审查。

Skill 本身并不是一个函数或工具调用，而是：

- 描述「**什么时候**」使用某些工具；
- 描述「**如何组合**」多个工具；
- 给出任务拆解、注意事项、业务语义和最佳实践。

### 2.2 Skill 包结构

对标 Anthropic 的 [skills 仓库](https://github.com/anthropics/skills) 和 hello-agents 总结，每个 Skill 作为一个目录：

```text
skills/frontend-design/
  ├── SKILL.md          # 核心定义（frontmatter + 详细指令）
  ├── scripts/...       # 可选脚本/辅助代码
  ├── docs/...          # 可选附加文档
  └── assets/...        # 可选资源（模板、示例等）
```

其中：

- **`SKILL.md`** 为必需文件，由：
  - 顶部 **YAML Frontmatter**（元数据）；
  - 下方 Markdown 正文（指令 + 工作流 + 示例等）构成。

### 2.3 渐进式披露（Progressive Disclosure）

结合 Anthropic Skills 与 hello-agents 的总结，Skill 内容加载分为三层：

1. **元数据（Frontmatter）**
   - Agent 启动时，仅解析 `SKILL.md` 的 YAML frontmatter；
   - System Prompt 或技能索引中，只注入 name/description/tags 等少量摘要（每个 Skill 数十~上百 token）。

2. **Skill 主体（指令正文）**
   - 当模型判断某个 Skill 与当前任务强相关时，才通过工具读取完整 `SKILL.md` 正文，注入当前对话上下文；
   - 包含任务拆解、工作流、SQL/代码模板等。

3. **附加资源（scripts / docs / assets）**
   - 仅在 Skill 指令/模型推理需要时，通过现有文件/脚本工具访问；
   - 不默认注入上下文，避免 token 爆炸。

这种设计的目标是：

- **降低初始上下文成本**（避免 MCP 那样一次性加载大量 schema）；
- **保留“无限扩展”能力**：通过脚本和资源文件承载更大规模的知识或数据。

---

## 3. 与现有 ReAct / Tools / MCP 架构的衔接

### 3.1 现有关键组件

- `tool` 包：
  - `Tool` struct（Name / Description / Parameters / Execute）；
  - `Registry` 管理工具集合；
  - 已有的业务工具如 `list_tables`、`describe_table`、`execute_read`、`execute_write` 等。
- `agent.ReActAgent`：
  - 基于 `ToolCallingModel.ChatWithTools` 的 ReAct 循环；
  - 每步由模型决定是否/如何调用工具。
- `templates` 包：
  - 负责把 `model.Model` + `memory.Memory` + `tool.Registry` + 中间件组装成 Handler；
  - 如 `NewDataQueryHandlerFromConfig`、`NewChatAgentHandlerFromConfig`、现有的 `NewMCPAgentHandler` 等。
- MCP 相关：
  - `tool/mcp.go` 中已经封装了 `metoro-io/mcp-golang` 与 `mark3labs/mcp-go` 两套客户端，通过 `McpConfig.Backend` 选择实现；
  - MCP 层已可作为 tools API 对外暴露「调用远端 MCP 工具」的能力。

### 3.2 Skills 的位置

在此架构上，Skills 作为一个**新增的“知识层”**：

```text
应用层 / 业务 Agent
  ↑  使用 Skills 指导工具调用
Skills 层（新）
  ↑  SKILL.md + scripts/docs/assets
工具层（现有 tool.Registry + MCP 工具）
  ↑  数据查询工具、MCP 工具、本地工具等
连接层（数据源/MCP/外部系统）
```

- Skills 不改变 Tools 的形态，也不替代 MCP；
- Skills 提供「操作手册」，告诉模型如何更好地使用这些工具。

在实现上：

- Skills 内容多半通过**现有的文件读取工具或轻量封装工具**访问；
- 业务 Agent 在判断需要技能时，通过工具调用加载 Skill 正文，并据此规划下一步工具调用。

---

## 4. 功能需求

### 4.1 构建阶段（Agent 启动）

1. **Skill 目录扫描**
   - 在配置层支持指定一个或多个 Skills 目录，例如：
     - `skills/`
     - `skills.d/` 等；
   - Agent 启动时，扫描这些目录下的 `**/SKILL.md` 文件。

2. **加载 Skill 元数据（Frontmatter）**
   - 仅解析 `SKILL.md` 顶部 YAML Frontmatter，至少包括：
     - `name`：Skill 唯一标识（kebab-case，如 `frontend-design`）；
     - `description`：简洁但精确地描述 Skill 做什么、什么时候使用；
     - `tags`：标签（如 `["database", "mysql"]`、`["frontend", "design"]`）；
     - 可选：`allowed_tools`、`version`、`author`、`required_context` 等；
   - 构建一个内存中的索引结构，如：

```go
type SkillMeta struct {
    Name        string
    Description string
    Tags        []string
    AllowedTools []string
    Path        string // SKILL.md 路径
}
var AllSkills []SkillMeta
```

3. **与 System Prompt 的集成**
   - 在构造特定 Agent 的 System Prompt 时（如 dataquery Agent、MCP Agent）：
     - 根据 Agent 类型或配置，从 `AllSkills` 中筛出相关技能（按 `tags` 或显式列表）；
     - 将它们的 `name/description` 压缩成少量文本注入 system 提示，例如：

> 你可以按需加载以下技能以增强能力（通过文件读取工具获取对应 SKILL.md 内容）：  
> - `frontend-design`：用于生成具有品牌感、差异化的前端页面与组件。  
> - `mysql-employees-analysis`：用于基于 MySQL employees 示例库做员工/薪资/部门分析。  

   - 控制总 token 预算，例如每个 Skill 1~2 行，总体不超过几百 token。

### 4.2 用户交互阶段

1. **技能匹配与选择**
   - 模型在看到：
     - 用户当前问题；
     - 历史上下文；
     - System Prompt 中列出的 Skills 摘要；
   - 自主决定是否需要某个 Skill，例如：

> 当前任务涉及前端设计 → 可以尝试加载 `frontend-design` 技能。  
> 当前任务是 MySQL 员工分析 → 考虑加载 `mysql-employees-analysis` 技能。  

2. **Skill 正文加载（第二层披露）**
   - 实现一个简单的「加载技能」工具，例如：

```go
// 伪代码：LoadSkill 工具
Name: "load_skill"
Params: { "name": string }
Execute: 根据 name 在 AllSkills 中查 Path，读取 SKILL.md 全文后返回。
```

   - 当模型决定使用某个 Skill 时，通过 `load_skill` 或底层文件读取工具获取 `SKILL.md` 正文；
   - Agent 将 Skill 正文作为工具 observation 注入 ReAct 对话，再发起下一轮模型调用，让模型基于 Skill 工作流规划后续工具调用。

3. **在 Skill 指导下调用现有 Tools/MCP**

   - Skill 文档中要明确：
     - 推荐工作流（步骤 1/2/3）；
     - 需要使用哪些工具，以及这些工具的参数约定；
     - 示例：如何多步查询/多次调用 MCP 或 dataquery 工具。
   - 模型加载 Skill 后，结合当前上下文，按文档指导依次调用：
     - 本地工具（如 `list_tables`、`execute_read`）；
     - MCP 工具（如 `mcp_call` 或具体 MCP 工具）。

4. **附加资源的按需加载**

   - Skill 可以引用同目录下其他文件（`scripts/`、`docs/`、`assets/`）；
   - 模型在 Skill 指令或自身推理需要时：
     - 使用已有文件读取工具获取附加文档；
     - 使用脚本执行工具（如 Bash/Python 工具）执行辅助脚本；
   - 不将脚本/大文档直接注入 system prompt，避免浪费上下文。

---

## 5. Skill 规范与编写要求

### 5.1 `SKILL.md` Frontmatter 规范

建议采用类似 Anthropic Skills 的 frontmatter 结构：

```yaml
---
name: mysql-employees-analysis
description: >
  将中文业务问题转换为 SQL 查询并分析 MySQL employees 示例数据库。
  适用于员工信息查询、薪资统计、部门分析、职位变动历史等场景。
version: 1.0.0
tags: [database, mysql, sql, employees, analysis]
allowed_tools: [list_tables, describe_table, execute_read]
author: your-name <you@example.com>
license: MIT
---
```

要求：

- **name**：全局唯一、kebab-case；
- **description**：完整说明：
  - Skill 做什么；
  - 何时适用；
  - 给 Agent 带来的独特能力；
- **tags**：用于匹配/过滤（例如 `"frontend"`, `"design"`, `"database"`, `"mysql"`）；
- **allowed_tools**：
  - 声明 Skill 期望/允许使用的工具列表；
  - 便于在执行层做安全校验与提示；
- **MCP 相关（可选）**：当 Skill 需要调用 MCP 时，可在 frontmatter 中声明 MCP 信息，便于运行时关联或校验：
  - `mcp_servers`：该 Skill 依赖的 MCP 服务 ID 列表（与全局配置中的 MCP 服务对应）；
  - `mcp_tools`：该 Skill 允许调用的 MCP 工具名列表（可选，用于白名单或提示）。
  - 示例：

```yaml
mcp_servers: [filesystem, github]
mcp_tools: [read_file, list_dir, get_repo_info]
```

- 其他字段（version/author/license）有助于管理和分享。

### 5.2 正文结构建议

参考 hello-agents 文档和 Anthropic Skills 实践，建议正文采用类似结构：

- `## 概述`：说明 Skill 的意图、数据源或系统背景；
- `## 前置条件`：说明使用此 Skill 前需要的配置（如“必须已配置 dataquery + MySQL 数据源 X”）；
- `## 工作流程`：用子标题/列表形式写清多步任务的通用执行流程；
- `## 常见模式/模板`：
  - 对数据类 Skill：给出常用 SQL 模板；
  - 对前端类 Skill：给出页面布局/组件组合建议；
- `## 最佳实践`：包括性能、安全、可维护性等；
- `## 示例`：典型用户问题与对应工具调用/结果风格；
- `## 故障排查`：常见错误与对应的排查思路。

### 5.3 编写原则

结合官方与社区经验：

1. **单一职责**：一个 Skill 专注一个领域/任务类型，避免“大而全”的通用 Skill。
2. **清晰触发条件**：description 和正文中明确 Skill 适用的“关键字/场景”，便于模型匹配。
3. **确定性优先**：复杂、关键操作尽量通过脚本/明确指令完成，减少 LLM 自由发挥导致的不确定性。
4. **分层组织知识**：
   - 高频、核心信息放在 `SKILL.md` 主体；
   - 低频、高级技巧放在附加文档，按需加载。

---

## 6. 安全策略与命名规范

### 6.1 安全策略

1. **工具白名单**
   - 使用 `allowed_tools` 声明此 Skill 打算使用的工具；
   - 在工具执行层可选做一次校验：若 Skill 尝试调用未在白名单的高危工具（如写库/删除类），可给出警告或拒绝。

2. **只读/写改区分**
   - 数据/资源修改类 Skill 必须在 description 和正文中明确标记风险；
   - 与 dataquery/MCP 写路径集成时，应复用现有「权限 + 确认 token」流程：
     - 例如，Skill 中只规划“提议写操作”，真正执行需用户确认。

3. **脚本安全**
   - `scripts/` 目录下的脚本执行能力应被视为高危；
   - 默认不暴露执行脚本工具，或需额外配置/权限控制；
   - 对生产环境建议仅启用经过审计的 Skills。

### 6.2 命名规范

- `name` 使用小写 kebab-case：
  - 示例：`frontend-design`、`mysql-employees-analysis`、`code-review-workflow`。
- 避免与工具名冲突：
  - Tool 名如 `list_tables`，Skill 名应为更高层次概念（如 `mysql-schema-exploration`）。
- 多团队/多组织时，可使用前缀区分：
  - `company-frontend-design`、`teamX-risk-evaluation`。

---

## 7. 配置与可扩展性

### 7.1 配置建议

在全局配置（如 `config.Config` 或新建 `SkillsConfig`）中增加：

- `SkillsDirs []string`：扫描的技能目录列表；
- `EnabledSkills []string`：可选白名单，如果非空则只启用指定技能；
- `DisabledSkills []string`：可选黑名单，便于临时关闭某些 Skills。
- **脚本执行（可选，参见 11.3、11.3.9）**：`allow_script_execution`、`script_allowed_extensions`、`script_timeout_seconds`；后续扩展可增加 `script_interpreters`（扩展名→解释器）、`enforce_skill_allowed_tools_for_script`（是否按 Skill 的 `allowed_tools` 做执行前校验）。
- **MCP 信息（可选）**：在 Skills 配置中可声明与 MCP 的关联，供「使用 MCP 的 Skill」或 Skills-aware Agent 使用：
  - `MCPServers` / `mcp_servers`：MCP 服务列表（如 endpoint、id、backend），与现有 MCP 客户端配置结构一致；
  - 当某 Skill 的 frontmatter 声明了 `mcp_servers` 时，运行时可根据全局 Skills 配置中的 MCP 信息解析出对应端点或客户端，确保该 Skill 可用到正确的 MCP 能力；
  - 若未在 Skills 配置中提供 MCP 信息，则依赖上层（如 Agent 或应用）已注入的 MCP 客户端/工具，Skill 仅做声明与白名单约束。

- **将 MCP 能力注册到上下文**：当 Skill 配置了 MCP 信息（全局 `Skills.MCPServers` 与/或 Skill frontmatter 中的 `mcp_servers`）时，**必须把对应 MCP 服务暴露的工具注册到 Agent 的上下文中**（即该 Agent 使用的 `tool.Registry`），模型在加载或使用该 Skill 时才能实际调用这些 MCP 工具。具体可采取以下方式之一（或组合）：
  - **构建时注册**：在构造 Skills-aware Handler 时，根据 `Skills.MCPServers` 与当前可见 Skill 的 `mcp_servers` 声明，为这些 MCP 服务创建客户端并将它们提供的工具统一注册到该 Handler 的 `tool.Registry`，使所有会话共享同一批 MCP 工具；
  - **按需注册（可选）**：当模型调用 `load_skill` 加载了声明 `mcp_servers` 的 Skill 时，若当前上下文中尚未注册该 Skill 依赖的 MCP 工具，则在此刻将对应 MCP 客户端/工具注册到当前会话的 Registry（需框架支持会话级或请求级动态注册），实现「仅在使用该 Skill 时才暴露其 MCP 能力」的渐进式披露。

无论采用哪种方式，均需保证：**Skill 一旦声明并配置了 MCP 信息，其依赖的 MCP 能力必须在模型可用的工具列表中**，否则 Skill 正文中描述的 MCP 调用无法被执行。

### 7.2 与 MCP 的协同（后续）

本需求主要聚焦于本地 Skills（文件系统）。后续可考虑：

- 通过 MCP server 暴露 Skills 列表与 `SKILL.md` 内容；
- 将「本地 Skills + 远端 Skills」统一收敛到一个 Skills 索引层；
- 利用 MCP 的能力访问远端技能仓库，与本地 Skills 一并管理（对标 [anthropics/skills](https://github.com/anthropics/skills) 这类公共仓库）。

---

## 8. 非功能需求

- **可扩展性**
  - 新增 Skill 只需在 Skills 目录中加入一个新文件夹，无需改动 Agent 逻辑；
  - 同一套 Agent 可根据配置启用不同 Skills 集合，适应不同业务场景。

- **可维护性**
  - 业务逻辑与领域知识更多存在于 SKILL.md 中，以 Markdown/文本方式维护；
  - 基础设施/工具层（dataquery/MCP/FS 工具）与 Skills 逻辑解耦。

- **上下文效率**
  - 通过渐进式披露，仅在需要时加载 Skill 正文与附加资源；
  - 避免像直接加载全部 MCP tools schema 那样占用大量 token。

- **生态兼容**
  - Skill 语义和结构尽量贴近 Anthropic 的 Skills 规范；
  - 便于未来直接复用/移植 [anthropics/skills](https://github.com/anthropics/skills) 中的一些技能（稍作路径适配即可）。

---

这份文档定义了在当前项目架构下引入 Skills 层的目标、概念、与 ReAct/Tools/MCP 的衔接方式，以及 Skill 的格式与编写规范。下文进一步给出**架构设计与功能设计**，便于直接落地实现。

---

## 9. 架构设计

### 9.1 包与模块划分

为最小化对现有架构的侵入性，引入一个新的 `skills` 包，并在现有 `tool` / `templates` / `config` 基础上做轻量扩展。

#### 9.1.1 `skills` 包（核心）

职责：发现、索引、加载 Skill 元数据与正文。

- `skills/meta.go`

```go
package skills

type SkillMeta struct {
    Name         string   // Skill 唯一标识（kebab-case）
    Description  string   // 简要描述
    Tags         []string // 用于匹配/过滤
    AllowedTools []string // 声明期望使用的工具（白名单）
    Path         string   // SKILL.md 路径
    // 可选：MCP 相关，由 frontmatter 解析
    MCPServers   []string // 依赖的 MCP 服务 ID 列表
    MCPTools     []string // 允许调用的 MCP 工具名（白名单/提示）
}
```

- `skills/index.go`

```go
type Index struct {
    skills []SkillMeta
    byName map[string]SkillMeta
}

func NewIndex(dirs []string, enabled, disabled []string) (*Index, error)
func (idx *Index) All() []SkillMeta
func (idx *Index) GetByName(name string) (SkillMeta, bool)
func (idx *Index) FilterByTags(tags []string) []SkillMeta
```

  - `NewIndex`：
    - 扫描 `dirs` 下的 `**/SKILL.md`；
    - 解析 frontmatter 得到 `SkillMeta`；
    - 按 `enabled/disabled` 做过滤；
    - 构建 `skills` 切片和 `byName` 索引。

- `skills/loader.go`

```go
func (idx *Index) LoadSkillBody(name string) (string, error)
// 内部：根据 name 从 byName 取 Path，读取 SKILL.md 全文并返回字符串
```

> `skills` 包不依赖 LLM/Agent，只做「文件系统 → 元数据/正文」的转换。

#### 9.1.2 `tool` 包扩展：加载 Skill 的工具

在 `tool` 包中新增 `skill_tools.go`，面向 ReActAgent 暴露“加载 Skill 正文”的工具。

```go
// RegisterLoadSkillTool 向 Registry 注册一个 load_skill 工具。
func RegisterLoadSkillTool(reg *Registry, idx *skills.Index) error {
    if reg == nil || idx == nil {
        return errors.New("load_skill: registry or index is nil")
    }
    return reg.Register(Tool{
        Name:        "load_skill",
        Description: "Load full SKILL.md content by skill name.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "name": map[string]any{
                    "type":        "string",
                    "description": "Skill name (kebab-case).",
                },
            },
            "required": []string{"name"},
        },
        Execute: func(ctx context.Context, params map[string]any) (any, error) {
            name, _ := params["name"].(string)
            if name == "" {
                return nil, errors.New("load_skill: name is required")
            }
            body, err := idx.LoadSkillBody(name)
            if err != nil {
                return nil, err
            }
            return body, nil
        },
    })
}
```

> 这样，任何使用 ReActAgent 的 Handler，只要在 Registry 中注册了 `load_skill`，模型就可以在 ReAct 流程中显式调用该工具来加载 Skill 正文。

#### 9.1.3 `config` 包扩展：Skills 配置

在 `config` 中增加 Skills 相关配置结构：

```go
type SkillsConfig struct {
    Dirs           []string `json:"skills_dirs" yaml:"skills_dirs"`
    EnabledSkills  []string `json:"enabled_skills" yaml:"enabled_skills"`
    DisabledSkills []string `json:"disabled_skills" yaml:"disabled_skills"`
    // 可选：MCP 服务列表，供声明了 mcp_servers 的 Skill 使用；与 MCP 客户端配置结构一致（如 endpoint、id、backend）。
    MCPServers     []MCPServerEntry `json:"mcp_servers" yaml:"mcp_servers"`
}

type Config struct {
    // ...
    Skills SkillsConfig `json:"skills" yaml:"skills"`
}
```

> 加载逻辑由 `skills.NewIndex` 使用；`Config` 本身只负责承载配置数据。**当 Skill 配置了 MCP 信息时，需把对应 MCP 能力注册到上下文**：在构造 Handler 时根据 `Skills.MCPServers` 与各 Skill 的 `mcp_servers` 声明，创建 MCP 客户端并将暴露的工具注册到该 Handler 使用的 `tool.Registry`，使模型在加载或使用该 Skill 时能实际调用这些 MCP 工具。

#### 9.1.4 `templates` 包扩展：Skills-aware Handler

在 `templates` 中新增“带 Skills 能力”的 Handler 构造函数，示例：

```go
// NewSkillsAwareChatHandlerFromConfig 构建一个支持 Skills 的对话 Handler。
func NewSkillsAwareChatHandlerFromConfig(cfg config.Config, skillsIdx *skills.Index, middlewareByName map[string]middleware.Middleware) (middleware.Handler, error)
```

内部逻辑：

1. 基本流程与 `NewChatAgentHandlerFromConfig` 相同：创建 `model.Model`、`memory.Memory`、中间件链；
2. 构造一个新的 `tool.Registry`，根据需要注册基础工具（如文件读取）；
3. **若配置中提供了 `Skills.MCPServers` 且存在声明了 `mcp_servers` 的 Skill**：根据 `MCPServers` 与各 Skill 的 `mcp_servers` 创建 MCP 客户端，将对应 MCP 工具注册到 `reg`，使这些能力在上下文中可用；
4. 调用 `tool.RegisterLoadSkillTool(reg, skillsIdx)` 注册 `load_skill`（及 `read_skill_file`）；
5. 根据 `skillsIdx.All()` 构造 Skill 摘要片段，拼进 System Prompt；
6. 使用 `agent.NewReActAgent(m, mem, reg, ...)` 构建 ReActAgent；
7. 返回将 System Prompt 注入到每次请求中的 `middleware.Handler`。

同理，可以为 dataquery Agent/MCP Agent 做一个 Skills-aware 版本，或在其 Handler 构造函数中增加可选 Skills 支持；**凡使用到配置了 MCP 的 Skill，均需在构造时或按需将对应 MCP 工具注册到该 Agent 的 Registry**。

---

## 10. 功能设计

### 10.1 构建阶段功能

#### 10.1.1 Skill 索引构建

**触发时机**：服务启动时 / Config 加载完毕后，由集成层（如 `templates` 或主入口）调用。

**步骤：**

1. 从 `cfg.Skills.Dirs` 拿到 Skills 目录列表；
2. 调用 `skills.NewIndex(cfg.Skills.Dirs, cfg.Skills.EnabledSkills, cfg.Skills.DisabledSkills)`：
   - 找到所有 `**/SKILL.md`；
   - 解析 frontmatter；
   - 过滤启用/禁用列表；
   - 构建 `Index`。
3. 将 `*skills.Index` 传入需要 Skills 的 Handler 构造函数（如 `NewSkillsAwareChatHandlerFromConfig`）。

#### 10.1.2 System Prompt 注入 Skills 摘要

在构造 Handler 时（例如 Skills-aware Chat Handler）：

1. 通过 `skillsIdx.All()` 获取所有启用的 Skill 元数据；
2. 可选：按 `tags` 或 Agent 类型过滤；
3. 生成一段系统提示片段，例如：

```text
【可用 Skills（按需加载）】
你可以按需加载以下技能以增强能力（通过调用 load_skill(name) 工具）：
- frontend-design：用于提升前端页面的设计感，避免统一的“AI 风格”界面。
- mysql-employees-analysis：用于分析 MySQL employees 示例库的员工、薪资和部门数据。
...
```

4. 在最终 System Prompt 中，将该片段拼接到其他系统提示（如角色、工作流说明）之后。

### 10.2 用户交互阶段功能

#### 10.2.1 技能匹配与调用流程

以 Skills-aware Chat Agent 为例，典型 ReAct 流程如下：

1. 用户提交问题；
2. ReActAgent 将：
   - System Prompt（含 Skills 摘要）；
   - 历史对话；
   - 当前用户消息；
   一并发送给模型；
3. 模型在“思考”中决定是否需要某个 Skill：
   - 若需要，第一步动作一般是调用 `load_skill`，例如：

```json
{"tool": "load_skill", "params": {"name": "frontend-design"}}
```

4. `load_skill` 工具执行：
   - 在 `skills.Index` 中找到对应 `SkillMeta.Path`；
   - 读取 `SKILL.md` 全文并返回字符串；
5. ReActAgent 将 Skill 正文作为 `tool` 消息注入对话，模型在下一轮调用时即可在上下文中“看到”完整 Skill 指令；
6. 模型根据 Skill 中描述的工作流与最佳实践，选择后续工具（如 dataquery/MCP 工具）完成任务；**若该 Skill 配置了 MCP 信息，其依赖的 MCP 工具应已注册到当前 Agent 的上下文中**，模型可直接按 Skill 正文中的说明调用这些 MCP 工具；
7. 重复 ReAct 循环，直到模型决定给出最终回答。

#### 10.2.2 使用 Skills 指导工具调用

Skill 文档中应明确：

1. 推荐工作流（步骤 1/2/3），例如：
   - 先调用 `list_tables`，再调用 `describe_table`，最后用 `execute_read` 编写 SQL；
2. 推荐工具及参数约定：
   - 对 dataquery：说明 `execute_read` 的 DSL 格式与注意事项；
   - 对 MCP：说明 `mcp_call` 的 `server/tool_name/args` 约定。

加载 Skill 后，模型应：

- 将当前用户问题 + Skill 指令 + 工具调用结果结合，按照 Skill 指导的步骤逐步执行；
- 在最终回复中，给出经过 Skill 指导后的自然语言总结，而不是简单转述工具原始结果。

#### 10.2.3 附加资源访问

若 Skill 引用额外文档或脚本（`docs/`、`scripts/`、`assets/`）：

- 可通过现有或新增的文件读取/脚本执行工具访问；
- 第一版可仅支持读取附加文档，脚本执行为后续增强（见下文实现状态）。
- Skill 中指令可写成：

> 若你需要了解更详细的 XXX 指南，可调用 `read_skill_file` 工具，传入 skill 名称与相对路径（如 `docs/advanced.md`）读取内容。  

模型即可在加载 Skill 后，按需调用 `read_skill_file(skill_name, path)` 进一步加载捆绑文档。

---

## 11. 实现状态：捆绑文档与脚本执行

### 11.1 捆绑文档（已实现）

- **工具**：`read_skill_file`（与 `load_skill` 一同注册到使用 Skills 的 Agent）。
- **能力**：按 Skill 名称 + 相对路径读取该 Skill 目录下的任意文件（如 `docs/advanced.md`、`assets/template.json`、`scripts/README.md`）。
- **安全**：路径限制在对应 Skill 根目录内，禁止通过 `..` 访问目录外文件。
- **实现位置**：`skills.LoadSkillFile`、`tool.RegisterReadSkillFileTool`；凡注册了 `load_skill` 的 Handler 会同时注册 `read_skill_file`。

在 SKILL.md 中可明确写：需要更多细节时，可调用 `read_skill_file(name, "docs/xxx.md")` 获取捆绑文档内容。

### 11.2 脚本执行（未实现，后续可选）

- **当前**：不提供「执行 Skill 目录下脚本」的工具；仅支持对 `scripts/` 下文件的**读取**（与 `docs/`、`assets/` 一致）。
- **原因**：脚本执行属高危能力，需沙箱、权限与配置开关（参见 6.1 脚本安全）；第一版聚焦「按需读取」即可满足大部分场景。
- **后续**：若需支持，可新增 `execute_skill_script(skill_name, script_relative_path, args)` 类工具，并配合配置项（如 `skills.allow_script_execution`）与 `allowed_tools` 白名单使用。

### 11.3 脚本执行详细设计

在启用脚本执行能力时，按以下设计实现，在安全可控前提下支持执行 Skill 捆绑的脚本。

#### 11.3.1 目标与范围

- **目标**：允许模型在遵循 Skill 指令的前提下，调用「执行某 Skill 目录下指定脚本」的工具，用于自动化、数据准备、本地校验等辅助能力。
- **范围**：仅限已索引的 Skill、且路径与扩展名符合白名单；执行受全局配置与（可选）Skill 级白名单约束。

#### 11.3.2 工具定义

- **工具名**：`execute_skill_script`
- **参数**：
  - `name`（必填）：Skill 名称（kebab-case），与 `load_skill` 一致。
  - `path`（必填）：脚本在 Skill 根目录下的相对路径，如 `scripts/run.sh`。
  - `args`（可选，后续扩展）：字符串数组，作为脚本参数传入；第一版可为空或不实现，由调用方在脚本内通过环境变量等间接传参。
- **行为**：在配置允许的前提下，将 `path` 解析为 Skill 根目录下的绝对路径，做安全校验后以规定方式执行，将 stdout/stderr 合并返回；执行失败或超时返回明确错误信息。

#### 11.3.3 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `skills.allow_script_execution` | bool | false | 为 true 时，Handler 才注册 `execute_skill_script` 工具；为 false 时工具不注册或执行时直接返回「脚本执行已禁用」类错误。 |
| `skills.script_allowed_extensions` | []string | 可选，如 `[".sh"]` | 允许执行的脚本扩展名白名单；未配置时建议仅允许 `.sh`。 |
| `skills.script_timeout_seconds` | int | 可选，如 30 | 单次脚本执行最大耗时（秒），超时则终止进程并返回错误。 |

配置来源：全局 Config 的 `SkillsConfig`（YAML/JSON 或环境变量覆盖），与 7.1 配置建议一致。

#### 11.3.4 路径与安全约束

- **路径解析**：
  - 由 `skills.Index` 根据 `name` 得到该 Skill 的 `SKILL.md` 路径，其父目录即为 Skill 根目录。
  - `path` 须为相对路径，经 `filepath.Clean` 规范化后不得包含 `..`，且最终绝对路径必须落在 Skill 根目录下（或其一子目录内）。
- **目录与扩展名白名单**：
  - **目录限制**：仅允许执行路径以 `scripts/` 为前缀的脚本（即脚本必须位于 Skill 的 `scripts/` 子目录下），禁止执行 `docs/`、`assets/` 或根目录下的可执行文件。
  - **扩展名白名单**：仅允许执行配置中 `script_allowed_extensions` 所列扩展名（如 `.sh`）；第一版可仅支持 `.sh`，由 `sh` 解释器执行。
- **存在性校验**：执行前需对解析后的绝对路径做 `os.Stat`，确认文件存在且为常规文件（非目录、非符号链接逃逸到目录外）。

#### 11.3.5 执行方式

- **工作目录**：进程的当前工作目录（cwd）设为该 Skill 的根目录，便于脚本内使用相对路径访问同 Skill 下的 `docs/`、`assets/`。
- **解释器**：第一版仅支持 `.sh`，使用系统 `sh`（或 `exec.Command("sh", scriptPath)`）执行，不传入用户可控参数；若后续支持 `args`，需按参数列表追加到命令行，并注意参数转义与注入风险。
- **超时**：使用 `context.WithTimeout`（或等价机制）限制执行时间，超时后终止子进程；超时值来自 `script_timeout_seconds`，未配置时使用合理默认（如 30 秒）。
- **环境变量**：不向脚本传递未过滤的用户输入；若需传递 Skill 名、路径等，可使用只读的环境变量（如 `SKILL_NAME`、`SKILL_ROOT`），由执行层固定设置。

#### 11.3.6 与 Skill 元数据的关系

- **allowed_tools**：Skill 的 frontmatter 中可声明 `allowed_tools: [..., execute_skill_script]`，表示该 Skill 允许使用脚本执行能力；执行层可**可选**做校验：仅当 Skill 的 `allowed_tools` 包含 `execute_skill_script` 时才允许执行该 Skill 的脚本，否则返回「该 Skill 未声明允许执行脚本」类错误。若不做校验，则仅依赖全局 `allow_script_execution` 与路径/扩展名白名单。
- **安全建议**：生产环境仅对经过审计的 Skill 开启脚本执行，并尽量结合 `enabled_skills` 白名单使用。

#### 11.3.7 错误与返回

- **成功**：返回脚本的 stdout 与 stderr 合并后的字符串（或结构化字段，由实现决定）。
- **失败**：返回明确错误信息，例如：
  - 配置未启用脚本执行；
  - Skill 不存在或未索引；
  - path 非法（含 `..`、不在 Skill 根下、不在 `scripts/` 下、扩展名不在白名单）；
  - 文件不存在或非普通文件；
  - 执行超时；
  - 进程启动或执行失败（可附带 stderr 或 exit status）。
- 实现上可对「执行失败但已有部分输出」的情况，将已捕获的输出与错误信息一并返回，便于排障。

#### 11.3.8 实现状态

- **第一版（当前）**：可实现为「仅支持 `scripts/` 下 `.sh`、全局开关 `allow_script_execution`、无 `args`、固定超时」的占位或最小实现；若暂不开放，则工具不注册或执行时统一返回「脚本执行已禁用」。
- **后续扩展**：按 11.3.9 设计实现多扩展名与解释器、可配置超时、以及按 Skill 的 `allowed_tools` 做执行前校验，与 6.1 脚本安全、7.1 配置建议保持一致。

#### 11.3.9 后续扩展设计（多扩展名 / 可配置超时 / allowed_tools 校验）

在首版脚本执行能力落地后，可按需实现下列扩展，并与 **6.1 脚本安全**、**7.1 配置建议** 保持一致。

**（1）更多扩展名与指定解释器**

- **目标**：除 `.sh` 外，支持 `.py` 等扩展名，并为每种扩展名指定解释器与默认参数，避免任意解释器带来的安全与可预测性问题。
- **配置扩展**（纳入 `SkillsConfig`，与 7.1 一致）：
  - **方案 A（扩展名 → 解释器映射）**：新增 `script_interpreters`，类型为「扩展名 → 解释器配置」的映射，例如：
    - `".sh"` → `{ "command": "sh", "args": [] }`
    - `".py"` → `{ "command": "python3", "args": ["-u"] }` 或 `{ "command": "/usr/bin/python3", "args": [] }`
  - **方案 B（扩展名白名单 + 固定映射）**：保持 `script_allowed_extensions` 白名单，在实现内维护「扩展名 → 解释器」的固定映射（如 `.sh`→`sh`、`.py`→`python3`），不暴露为配置；仅通过白名单控制允许的扩展名。
- **执行层**：根据脚本路径的扩展名查找解释器配置；使用 `exec.Command(interpreter, append(args, scriptPath)...)` 执行；不将未过滤的用户输入传入解释器参数（`args` 若支持，需严格转义与长度限制）。
- **与 6.1 的衔接**：脚本执行仍视为高危，仅对 `script_allowed_extensions` 内且已配置/映射的解释器执行；生产环境仅启用经审计的 Skills（6.1 第三点）。

**（2）可配置超时**

- **目标**：不同环境或不同脚本类型可配置不同的执行超时时间，与 7.1 中「配置集中、可覆盖」一致。
- **配置**：已有 `skills.script_timeout_seconds`（11.3.3）；扩展设计约定：
  - 未配置或 ≤0 时使用默认值（如 30 秒）；
  - 可设上限（如 300 秒），防止配置错误导致长时间占用；
  - 若后续支持按扩展名或 Skill 差异化超时，可在 `script_interpreters` 每项或 Skill 元数据中增加可选 `timeout_seconds`，执行时优先取该值，否则取全局 `script_timeout_seconds`。
- **执行层**：`context.WithTimeout(ctx, time.Duration(script_timeout_seconds)*time.Second)`；超时后终止子进程并返回明确「执行超时」错误（11.3.7）。

**（3）按 Skill 的 `allowed_tools` 做执行前校验**

- **目标**：与 6.1「工具白名单」一致：仅当 Skill 在 frontmatter 中声明了 `allowed_tools` 包含 `execute_skill_script` 时，才允许执行该 Skill 目录下的脚本，否则拒绝并返回明确错误。
- **配置**：新增全局开关，例如 `skills.enforce_skill_allowed_tools_for_script`（bool，默认 false）：
  - 为 **false**：仅依赖全局 `allow_script_execution` 与路径/扩展名白名单，不校验 Skill 的 `allowed_tools`（与首版行为一致）；
  - 为 **true**：在执行前读取该 Skill 的 `SkillMeta.AllowedTools`，若未包含 `execute_skill_script`，则直接返回「该 Skill 未声明允许执行脚本（allowed_tools 需包含 execute_skill_script）」类错误，不执行脚本。
- **与 6.1 的衔接**：6.1 规定「在工具执行层可选做一次校验：若 Skill 尝试调用未在白名单的高危工具，可给出警告或拒绝」；脚本执行属高危，故将「Skill 是否声明 `execute_skill_script`」作为可选强校验，由配置控制，便于生产收紧策略。
- **与 7.1 的衔接**：上述开关纳入 `SkillsConfig`，与 `EnabledSkills`/`DisabledSkills` 等一起由 YAML/JSON 或环境变量加载与覆盖。

**（4）实施顺序与兼容性**

- 可配置超时（2）可与首版同步实现（仅读取已有或新增的 `script_timeout_seconds`）。
- 多扩展名与解释器（1）、allowed_tools 校验（3）建议在首版稳定后迭代：先实现（3）并默认关闭，再实现（1）并扩展 `script_allowed_extensions` 与解释器映射；配置项命名与 7.1 保持一致（如 `skills.script_interpreters`、`skills.enforce_skill_allowed_tools_for_script`）。

---

## 12. 落地步骤建议（MVP）

为降低一次性改动风险，建议按以下顺序迭代实现：

1. **实现 `skills.Index`（本地 SKILL.md + frontmatter 解析）**
   - 支持从配置指定的 `SkillsDirs` 中扫描 Skills；
   - 暂不考虑远端 Skills。

2. **在 `tool` 包中实现 `RegisterLoadSkillTool`**
   - 依赖 `*skills.Index`；
   - 注册 `load_skill` 工具。

3. **实现一个简单的 Skills-aware Chat Handler**
   - 例如 `NewSkillsAwareChatHandlerFromConfig`：
     - 使用 `skills.Index` 生成 Skills 摘要片段；
     - 在 System Prompt 注入该片段；
     - 在 `tool.Registry` 中注册 `load_skill`；
     - 使用 `agent.NewReActAgent` 组装最终 Handler。

4. **编写 1～2 个示例 Skill**
   - `skills/frontend-design/SKILL.md`：可以参考 Anthropic 前端设计 Skill 博客；
   - `skills/mysql-employees-analysis/SKILL.md`：复用 dataquery 相关经验。

5. **人工对话验证闭环**
   - 启动 Skills-aware Chat Handler；
   - 发送适合触发 Skill 的问题（前端设计/员工分析等）；
   - 观察模型是否：
     - 调用了 `load_skill`；
     - 使用 Skill 内容指导后续工具调用（或直接给出更高质量回答）。

6. **后续增强**
   - 在 dataquery/MCP Agent 中也接入 Skills 索引与 `load_skill`；
   - 引入 `scope` 字段（指定 Skill 适用的 Agent 类型），提升匹配精度；
   - 与 MCP server 协作，将远端 Skills 纳入统一索引。

通过以上步骤，可以在不破坏现有架构的前提下，为系统增加一个可扩展、可复用的 Skills 知识层，使 Agent 在使用 MCP/Tools 时具备更强的“会用好”能力。  


