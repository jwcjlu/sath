package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v2"
)

// Index 维护 Skill 元数据的内存索引。
type Index struct {
	skills []SkillMeta
	byName map[string]SkillMeta
}

// NewIndex 扫描给定目录中的 SKILL.md 文件，解析 frontmatter 构建索引。
// - dirs 为空时返回一个空索引且不报错；
// - enabled 非空时视为白名单，仅保留列表中的技能；
// - disabled 为黑名单，用于排除部分技能。
func NewIndex(dirs []string, enabled, disabled []string) (*Index, error) {
	idx := &Index{
		skills: make([]SkillMeta, 0),
		byName: make(map[string]SkillMeta),
	}
	if len(dirs) == 0 {
		return idx, nil
	}

	enabledSet := toSet(enabled)
	disabledSet := toSet(disabled)

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		root := dir
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				// 目录不可读等场景直接返回错误，由调用方处理。
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Base(path), "SKILL.md") {
				meta, ok, perr := parseSkillFrontmatter(path)
				if perr != nil {
					return perr
				}
				if !ok {
					return nil
				}
				// 白名单过滤：若 enabled 非空且不在白名单中，则跳过。
				if len(enabledSet) > 0 {
					if _, hit := enabledSet[meta.Name]; !hit {
						return nil
					}
				}
				// 黑名单过滤：在黑名单中的直接跳过。
				if _, banned := disabledSet[meta.Name]; banned {
					return nil
				}
				meta.Path = path
				// 后扫描到的相同 name 可覆盖之前的定义，但实际应避免重名。
				idx.byName[meta.Name] = meta
				idx.skills = append(idx.skills, meta)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return idx, nil
}

// All 返回当前索引中的全部 Skill 元数据切片（拷贝）。
func (idx *Index) All() []SkillMeta {
	if idx == nil {
		return nil
	}
	out := make([]SkillMeta, len(idx.skills))
	copy(out, idx.skills)
	return out
}

// GetByName 根据 Skill 名称获取元数据。
func (idx *Index) GetByName(name string) (SkillMeta, bool) {
	if idx == nil {
		return SkillMeta{}, false
	}
	meta, ok := idx.byName[name]
	return meta, ok
}

// FilterByTags 根据标签过滤技能，只要有任意标签命中即可返回。
// tags 为空时返回全部技能。
func (idx *Index) FilterByTags(tags []string) []SkillMeta {
	if idx == nil {
		return nil
	}
	if len(tags) == 0 {
		return idx.All()
	}
	tagSet := toSet(tags)
	var out []SkillMeta
	for _, m := range idx.skills {
		for _, t := range m.Tags {
			if _, ok := tagSet[t]; ok {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// FilterByScope 按 scope 过滤技能。
// 约定：
//   - scope 为空：返回全部技能；
//   - Skill.Scopes 为空：视为通用技能，对所有 scope 可见；
//   - 否则仅当 scope 命中 Skill.Scopes 时返回。
func (idx *Index) FilterByScope(scope string) []SkillMeta {
	if idx == nil {
		return nil
	}
	if scope == "" {
		return idx.All()
	}
	var out []SkillMeta
	for _, m := range idx.skills {
		if len(m.Scopes) == 0 {
			out = append(out, m)
			continue
		}
		for _, s := range m.Scopes {
			if s == scope {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// MCPServerIDsFromSkills 返回在给定 scope 下、被至少一个 Skill 声明的 MCP 服务 ID 列表（去重）。
// 用于决定需要将哪些 MCP 服务注册到 Agent 上下文。
func (idx *Index) MCPServerIDsFromSkills(scope string) []string {
	if idx == nil {
		return nil
	}
	skills := idx.FilterByScope(scope)
	seen := make(map[string]struct{})
	var ids []string
	for _, m := range skills {
		for _, id := range m.MCPServers {
			if id == "" {
				continue
			}
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// AnyAllowsTool 在给定 scope 下，是否存在至少一个 Skill 的 AllowedTools 包含指定工具。
func (idx *Index) AnyAllowsTool(scope, toolName string) bool {
	if idx == nil || toolName == "" {
		return false
	}
	candidates := idx.FilterByScope(scope)
	for _, m := range candidates {
		for _, t := range m.AllowedTools {
			if t == toolName {
				return true
			}
		}
	}
	return false
}

// parseSkillFrontmatter 解析 SKILL.md 顶部的 YAML frontmatter。
// 若文件不包含合法 frontmatter，则返回 ok=false。
func parseSkillFrontmatter(path string) (SkillMeta, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SkillMeta{}, false, err
	}
	content := string(data)
	content = strings.TrimLeft(content, "\ufeff") // 去除 BOM

	if !strings.HasPrefix(content, "---") {
		// 没有 frontmatter，视为无效 Skill 定义。
		return SkillMeta{}, false, nil
	}

	// 寻找第二个 '---' 作为 frontmatter 结束标记。
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return SkillMeta{}, false, errors.New("skills: invalid SKILL.md frontmatter in " + path)
	}
	rawYAML := parts[1]

	type frontmatter struct {
		Name         string      `yaml:"name"`
		Description  string      `yaml:"description"`
		Tags         []string    `yaml:"tags"`
		Scope        interface{} `yaml:"scope"` // 兼容字符串或字符串数组
		AllowedTools []string    `yaml:"allowed_tools"`
		McpServers   interface{} `yaml:"mcp_servers"` // 兼容字符串或字符串数组
		McpTools     interface{} `yaml:"mcp_tools"`   // 兼容字符串或字符串数组
	}

	var fm frontmatter
	if err = yaml.Unmarshal([]byte(rawYAML), &fm); err != nil {
		return SkillMeta{}, false, err
	}
	if fm.Name == "" {
		return SkillMeta{}, false, errors.New("skills: frontmatter missing name in " + path)
	}

	// 归一化 scope → []string
	var scopes []string
	switch v := fm.Scope.(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			scopes = []string{s}
		}
	case []interface{}:
		for _, it := range v {
			if s, ok := it.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					scopes = append(scopes, s)
				}
			}
		}
	case []string:
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				scopes = append(scopes, s)
			}
		}
	}

	mcpServers := normalizeStringList(fm.McpServers)
	mcpTools := normalizeStringList(fm.McpTools)

	return SkillMeta{
		Name:         fm.Name,
		Description:  strings.TrimSpace(fm.Description),
		Tags:         fm.Tags,
		Scopes:       scopes,
		AllowedTools: fm.AllowedTools,
		MCPServers:   mcpServers,
		MCPTools:     mcpTools,
	}, true, nil
}

// normalizeStringList 将 frontmatter 中的 interface{} 归一化为 []string（兼容 string、[]string、[]interface{}）。
func normalizeStringList(v interface{}) []string {
	var out []string
	switch val := v.(type) {
	case string:
		if s := strings.TrimSpace(val); s != "" {
			out = []string{s}
		}
	case []interface{}:
		for _, it := range val {
			if s, ok := it.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
	case []string:
		for _, s := range val {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func toSet(list []string) map[string]struct{} {
	m := make(map[string]struct{}, len(list))
	for _, s := range list {
		if s == "" {
			continue
		}
		m[s] = struct{}{}
	}
	return m
}
