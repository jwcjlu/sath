## Skills 开发任务计划（基于 `skills-requirements.md`）

本计划将 Skills 能力需求拆分为若干可实施的开发任务，按阶段推进，方便迭代与回滚。

---

## 阶段一：基础设施与配置层

### 任务 1：定义 Skills 配置结构

- **目标**：在全局配置中引入 Skills 相关字段，为后续索引与 Handler 集成准备数据来源。
- **工作内容**：
  - 在 `config` 包中新增：
    - `type SkillsConfig struct { Dirs []string; EnabledSkills []string; DisabledSkills []string; MCPServers []MCPServerEntry }`（其中 `MCPServers` 为可选，与现有 MCP 客户端配置结构一致）；
    - 在 `Config` 结构体中增加 `Skills SkillsConfig` 字段（支持 json/yaml 标签）。
  - 确认配置文件（YAML/JSON）中可正确加载上述字段。
- **验收标准**：
  - 应用启动时能成功解析并访问 `cfg.Skills`；
  - 未配置 Skills 时保持兼容（零值不影响现有逻辑）；
  - 可选配置 `skills.mcp_servers` 时能正确解析，供「Skill 配置了 MCP 时注册到上下文」使用。

### 任务 2：实现 `skills` 包的元数据模型

- **目标**：定义 Skill 元数据结构，统一描述 name/description/tags 等信息。
- **工作内容**：
  - 新建 `skills` 包与 `meta.go` 文件；
  - 实现 `SkillMeta` 结构体，字段包括：`Name, Description, Tags, AllowedTools, Path`；可选 MCP 相关：`MCPServers []string`（依赖的 MCP 服务 ID）、`MCPTools []string`（允许调用的 MCP 工具名，用于白名单/提示）。
- **验收标准**：
  - 代码编译通过，无引用循环；
  - 其他包可正常引用 `skills.SkillMeta`。

### 任务 3：实现 `skills.Index` 与 Frontmatter 解析

- **目标**：支持从文件系统扫描 `SKILL.md`，解析 frontmatter，构建内存索引。
- **工作内容**：
  - 新建 `index.go`，实现：
    - `type Index struct { skills []SkillMeta; byName map[string]SkillMeta }`；
    - `NewIndex(dirs []string, enabled, disabled []string) (*Index, error)`；
    - `All() []SkillMeta`、`GetByName(name string) (SkillMeta, bool)`、`FilterByTags(tags []string) []SkillMeta`。
  - 逻辑细节：
    - 递归扫描 `dirs` 下的 `**/SKILL.md`；
    - 解析 `SKILL.md` 顶部 YAML frontmatter，映射到 `SkillMeta`（含可选 `mcp_servers`、`mcp_tools`）；
    - 根据 `enabled/disabled` 做白名单/黑名单过滤；
    - 填充 `skills` 切片与 `byName` 索引。
  - 选型：复用现有 YAML 库或引入轻量依赖。
- **验收标准**：
  - 针对临时目录的单元测试通过（包含多 Skill、过滤逻辑、异常 frontmatter 等场景）；
  - `NewIndex` 对于无 Skill 场景表现良好（返回空索引而非错误）。

### 任务 4：实现 Skill 正文加载 `LoadSkillBody`

- **目标**：在需要时按名称加载完整 `SKILL.md` 文本，支持渐进式披露第二层。
- **工作内容**：
  - 在 `loader.go` 中为 `Index` 实现：
    - `LoadSkillBody(name string) (string, error)`；
  - 根据 `name` 从 `byName` 获取 `SkillMeta.Path`，读取文件内容并返回。
- **验收标准**：
  - 单元测试验证：给定已索引的 Skill 名称，能读取到完整 Markdown 文本；
  - 对不存在的名称返回明确错误，不 panic。

---

## 阶段二：工具与 Agent 集成（MVP）

### 任务 5：在 `tool` 包中注册 `load_skill` 与 `read_skill_file` 工具

- **目标**：为 ReActAgent 提供「加载 Skill 正文」与「读取 Skill 捆绑文档」的工具入口。
- **工作内容**：
  - 新增 `tool/skill_tools.go` 文件；
  - 实现 `RegisterLoadSkillTool(reg *Registry, idx *skills.Index) error`：
    - 校验 `reg` 与 `idx` 非空；
    - 注册名称为 `load_skill` 的 `Tool`（参数 `name`，执行时调用 `idx.LoadSkillBody(name)`）；
    - 同时调用 `RegisterReadSkillFileTool(reg, idx)` 注册 `read_skill_file`：参数 `name`、`path`（相对路径，如 `docs/advanced.md`），执行时调用 `idx.LoadSkillFile(name, path)`，路径限制在 Skill 目录内、禁止 `..` 逃逸。
  - 在 `skills` 包中实现 `LoadSkillFile(name, relativePath string) (string, error)`（见任务 4 扩展或独立 loader 能力）。
- **验收标准**：
  - 工具列表中可以看到 `load_skill` 与 `read_skill_file`；
  - 调用 `read_skill_file(skill_name, "docs/xxx.md")` 能正确返回 Skill 目录下文件内容；对 `..` 路径返回错误。

### 任务 6：在 `templates` 中实现 Skills-aware Chat Handler

- **目标**：提供一个示例 Handler，将 Skills 能力接入到对话 Agent 中。
- **工作内容**：
  - 新增或扩展一个构造函数，例如：
    - `NewSkillsAwareChatHandlerFromConfig(cfg config.Config, skillsIdx *skills.Index, middlewareByName map[string]middleware.Middleware) (middleware.Handler, error)`；
  - 内部流程：
    - 使用现有逻辑创建 `model.Model`、`memory.Memory`、中间件链；
    - 构造新的 `tool.Registry`，注册基础工具（如文件读取）；
    - **MCP 注册时机**：不在请求开始时全量注册；仅在模型**明确使用某 Skill**（调用 `load_skill(name)`）时，将该 Skill 声明的 `mcp_servers` 对应的 MCP 工具注册到当前请求的 `reg`（参见需求 7.1「将 MCP 能力注册到上下文」）；
    - 调用 `tool.RegisterLoadSkillTool(reg, skillsIdx, mcpServers)` 注册 `load_skill` 与 `read_skill_file`，并传入 `cfg.Skills.MCPServers` 供 `load_skill` 执行时按需注册 MCP；
    - 调用 Skills 摘要生成函数（见任务 7），将文本拼接到 System Prompt；
    - 使用 `agent.NewReActAgent` 生成 ReActAgent，封装成 `middleware.Handler` 返回。
- **验收标准**：
  - 能通过配置启用/禁用该 Handler；
  - 在对话日志中能看到 `load_skill` 被模型调用并返回文本。

### 任务 7：System Prompt 中注入 Skills 摘要逻辑

- **目标**：在 System Prompt 中以低成本形式暴露可用 Skills，让模型知道“有哪些技能可用”。
- **工作内容**：
  - 在 `templates` 或 `skills` 包中实现一个辅助函数，例如：
    - `BuildSkillsSummary(skills []skills.SkillMeta, maxCount int) string`；
  - 逻辑：
    - 可按标签或 Agent 类型过滤一部分 Skill；
    - 控制数量或总长度（如最多 N 个 Skill，每个 1～2 行）；
    - 按文档给出的风格生成文案，例如：
      - 「你可以按需加载以下技能以增强能力（通过调用 load_skill(name) 工具）：…」。
- **验收标准**：
  - System Prompt 中出现预期格式的 Skills 摘要；
  - 调整 Skills 配置（启用/禁用）会反映到摘要内容中。

---

## 阶段三：示例 Skill 与端到端验证

### 任务 8：编写示例 Skills 目录与 SKILL.md

- **目标**：提供实际可用的示例 Skill，验证索引与加载逻辑。
- **工作内容**：
  - 新建目录结构，例如：
    - `skills/frontend-design/SKILL.md`；
    - `skills/mysql-employees-analysis/SKILL.md`。
  - 按文档中的 Frontmatter 与正文规范编写内容：
    - 定义 `name/description/tags/allowed_tools`；可选 `scope`、`mcp_servers`、`mcp_tools`；
    - 编写概述、工作流程、最佳实践、示例等章节。
- **验收标准**：
  - 通过 `skills.NewIndex` 能正确扫描并解析两个示例 Skill；
  - 通过 `load_skill` 工具能读取到它们的完整正文。

### 任务 9：端到端人工验证闭环

- **目标**：从配置加载 → 索引构建 → System Prompt 注入 → ReAct 调用 `load_skill` → 使用 Skill 指南，验证整体链路。
- **工作内容**：
  - 编写或调整配置，开启 Skills：
    - `skills_dirs` 指向示例 Skills 目录；
    - 可选配置 enabled/disabled 列表。
  - 启动 Skills-aware Chat Handler；
  - 设计两类人工测试对话：
    - 前端设计相关问题，预期触发 `frontend-design`；
    - MySQL employees 数据分析相关问题，预期触发 `mysql-employees-analysis`。
  - 观察与记录：
    - 模型是否在合适时机调用 `load_skill`；
    - 加载 Skill 后，工具调用与最终回答质量是否符合预期。
- **验收标准**：
  - 至少一个场景能稳定复现完整链路；
  - 如有问题（例如模型不愿意调用 `load_skill`），调整 prompt 或工具描述并记录经验。

---

## 阶段四：增强与与其他 Agent 的融合（可选/后续）

### 任务 10：将 Skills 接入 dataquery Agent / MCP Agent（可选）

- **目标**：让 dataquery 与 MCP 等业务 Agent 也能使用 Skills 作为知识层。
- **工作内容**：
  - 在相应 Handler 构造函数中增加可选 Skills 支持：
    - 复用已有 `SkillsConfig` 与 `skills.Index`；
    - 复用 `load_skill` 工具注册逻辑；
    - 为不同 Agent 构造更贴合场景的 Skills 摘要（如仅展示与数据库/MCP 相关的 Skills）。
- **验收标准**：
  - dataquery/MCP Agent 的 System Prompt 中也能看到对应 Skills 摘要；
  - 在这些 Agent 上同样可以通过 `load_skill` 使用相关 Skill。

### 任务 11：增加 Skill 范围 `scope` / 精细匹配

- **目标**：提升 Skill 与 Agent/任务的匹配精度，避免无关技能干扰。
- **工作内容**：
  - 在 `SKILL.md` Frontmatter 中新增可选字段，如 `scope` 或更细标签；
  - 在 `skills.Index` 与摘要生成逻辑中，基于 `scope`/tags 过滤不适用的 Skills；
  - 为不同 Agent 预设默认适用 scope。
- **验收标准**：
  - 不同 Agent 只展示与自己相关的 Skills，摘要长度受控；
  - 扩展新 Skill 时，仅通过配置 scope 即可控制可见范围。

### 任务 12：安全与工具白名单增强

- **目标**：在需要时加强对高危工具的约束，结合 Skills 的 `allowed_tools` 提升安全性。
- **工作内容**：
  - 设计可选的执行期校验策略：
    - 当对话中已经加载了某个 Skill 时，对敏感工具调用（写库/删除等）做白名单校验；
    - 若不在 Skill 的 `allowed_tools` 内，可以：
      - 打日志并放行，或
      - 返回警告/错误，要求额外确认。
  - 提供配置开关，用于启用/关闭这类安全增强。
- **验收标准**：
  - 在开启安全策略时，高危工具调用行为符合预期（被限制或需要更高门槛）；
  - 默认关闭时不影响现有业务。

### 任务 13：Skills 配置 MCP 信息并将 MCP 能力注册到上下文

- **目标**：支持在 Skills 中配置 MCP 信息，并保证当 Skill 声明了 MCP 时，对应 MCP 工具已注册到 Agent 上下文中，模型可实际调用。
- **工作内容**：
  - **配置与元数据**：在 `SkillsConfig` 中支持 `MCPServers`（或 `mcp_servers`）；在 `SkillMeta` 与 frontmatter 解析中支持 `mcp_servers`、`mcp_tools`（见任务 1、2、3 的扩展）。
  - **注册到上下文**：在构造 Skills-aware / dataquery / MCP 等 Handler 时，若存在 `Skills.MCPServers` 且当前可见 Skill 中有声明 `mcp_servers`，则根据配置创建 MCP 客户端，并将这些 MCP 服务暴露的工具注册到该 Handler 使用的 `tool.Registry`；或实现按需注册（在 `load_skill` 时若该 Skill 声明了 `mcp_servers` 且尚未注册，则注册对应 MCP 工具）。
  - 保证：Skill 一旦声明并配置了 MCP 信息，其依赖的 MCP 能力必须在模型可用的工具列表中。
- **验收标准**：
  - 配置 `skills.mcp_servers` 且某 Skill frontmatter 含 `mcp_servers` 时，该 Agent 的工具列表中包含对应 MCP 工具；
  - 模型加载该 Skill 后能成功调用其中描述的 MCP 工具。

### 任务 14：Skill 捆绑文档读取（read_skill_file）（已实现可标为完成）

- **目标**：支持按需读取 Skill 目录下的捆绑文件（`docs/`、`assets/`、`scripts/` 等），仅读取不执行。
- **工作内容**：
  - 在 `skills` 包实现 `LoadSkillFile(name, relativePath string) (string, error)`，路径限制在 Skill 根目录内、禁止 `..`；
  - 在 `tool` 包实现 `RegisterReadSkillFileTool(reg, idx)`，注册 `read_skill_file(skill_name, path)`；与 `load_skill` 一同注册到使用 Skills 的 Handler。
- **验收标准**：
  - 模型可调用 `read_skill_file` 获取 Skill 下 `docs/advanced.md` 等文件内容；
  - 对 `..` 或越界路径返回错误。  
（若已实现，本任务可标为已完成。）

### 任务 15：脚本执行（后续可选）

- **目标**：可选支持执行 Skill 目录下脚本（如 `scripts/`），需沙箱与配置开关。
- **工作内容**：
  - 设计 `execute_skill_script(skill_name, script_relative_path, args)` 类工具及配置项（如 `skills.allow_script_execution`）；
  - 与安全策略、`allowed_tools` 白名单配合。
- **验收标准**：在开启且白名单允许时，可安全执行指定脚本；默认关闭不影响现有行为。  
（需求文档约定第一版不实现，仅做任务预留。）

---

## 推荐实施顺序总结

1. **优先完成阶段一、二（任务 1～7）**：拿到可运行的 Skills-aware Chat Handler 与 `load_skill`、`read_skill_file` 工具；若需 MCP 与 Skill 联动，在任务 6 中纳入「MCP 能力注册到上下文」（或单独做任务 13）。
2. **随后推进阶段三（任务 8～9）**：通过示例 Skill 完成端到端验证闭环。
3. **视实际需求实施阶段四（任务 10～15）**：融入 dataquery/MCP Agent；scope 精细匹配；安全白名单；**Skills 配置 MCP 并将 MCP 注册到上下文（任务 13）**；捆绑文档（任务 14，通常已随任务 5 完成）；脚本执行（任务 15，后续可选）。

