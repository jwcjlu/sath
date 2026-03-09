package skills

// SkillMeta 描述一个 Skill 的基础元数据，由 SKILL.md 的 frontmatter 解析而来。
type SkillMeta struct {
	// Name 是 Skill 的唯一标识，推荐使用 kebab-case（例如 "frontend-design"）。
	Name string
	// Description 为简要描述，说明 Skill 做什么、何时使用。
	Description string
	// Tags 用于按领域或场景过滤 Skill（如 "database"、"frontend" 等）。
	Tags []string
	// Scopes 指定此 Skill 适用的 Agent 范围，如 "chat"、"dataquery"、"mcp" 等。
	Scopes []string
	// AllowedTools 声明此 Skill 期望或允许使用的工具列表，便于执行层做安全控制。
	AllowedTools []string
	// Path 为 SKILL.md 在文件系统中的绝对或相对路径，用于后续加载正文。
	Path string
	// MCPServers 该 Skill 依赖的 MCP 服务 ID 列表，与全局配置中的 MCP 服务对应。
	MCPServers []string
	// MCPTools 该 Skill 允许调用的 MCP 工具名列表（可选，用于白名单或提示）。
	MCPTools []string
}
