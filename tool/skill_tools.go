package tool

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sath/skills"
)

// McpServerEntry 描述一个可用的 MCP 服务配置（与 config.MCPServerEntry 对齐），用于在 load_skill 时按 Skill 声明按需注册。
type McpServerEntry struct {
	Endpoint string
	Id       string
	Backend  string
}

// RegisterLoadSkillTool 向 Registry 注册用于加载 Skill 正文的工具。当某 Skill 被加载且其 frontmatter 声明了 mcp_servers 时，
// 会按 mcpServers 配置将该 Skill 声明的 MCP 能力注册到当前 Registry，使后续步骤可调用对应 MCP 工具。
// mcpServers 可为 nil/空，此时仅加载正文，不注册 MCP。
func RegisterLoadSkillTool(reg *Registry, idx *skills.Index, mcpServers []McpServerEntry) error {
	if reg == nil {
		return errors.New("load_skill: registry is nil")
	}
	if idx == nil {
		return errors.New("load_skill: index is nil")
	}

	err := reg.Register(Tool{
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
			raw, ok := params["name"]
			if !ok {
				return nil, errors.New("load_skill: name is required")
			}
			name, _ := raw.(string)
			if name == "" {
				return nil, errors.New("load_skill: name is empty")
			}
			body, err := idx.LoadSkillBody(name)
			if err != nil {
				return nil, err
			}
			// 明确使用该 Skill 时，将该 Skill 声明的 MCP 能力注册到当前上下文（仅注册配置中存在的服务）。
			meta, hasMeta := idx.GetByName(name)
			if hasMeta && len(meta.MCPServers) > 0 && len(mcpServers) > 0 {
				byID := make(map[string]McpServerEntry)
				for _, e := range mcpServers {
					if e.Id != "" {
						byID[e.Id] = e
					}
				}
				for _, id := range meta.MCPServers {
					if e, ok := byID[id]; ok {
						RegisterMcpTool(reg, &McpConfig{
							Endpoint: e.Endpoint,
							Id:       e.Id,
							Backend:  e.Backend,
						})
					}
				}
			}
			return body, nil
		},
	})
	if err != nil {
		return err
	}
	return RegisterReadSkillFileTool(reg, idx)
}

// RegisterReadSkillFileTool 向 Registry 注册用于读取 Skill 捆绑文档的工具。
// 模型可据此按需加载 Skill 目录下的 docs/、assets/、scripts/ 等文件内容（仅读取，不执行）。
func RegisterReadSkillFileTool(reg *Registry, idx *skills.Index) error {
	if reg == nil {
		return errors.New("read_skill_file: registry is nil")
	}
	if idx == nil {
		return errors.New("read_skill_file: index is nil")
	}
	return reg.Register(Tool{
		Name:        "read_skill_file",
		Description: "Read a file bundled with a Skill by skill name and relative path (e.g. docs/advanced.md, assets/template.json). Path must be under the skill directory; only reading is supported, no script execution.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Skill name (kebab-case).",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Relative path under the skill directory, e.g. docs/advanced.md, assets/example.json.",
				},
			},
			"required": []string{"name", "path"},
		},
		Execute: func(ctx context.Context, params map[string]any) (any, error) {
			name, _ := params["name"].(string)
			if name == "" {
				return nil, errors.New("read_skill_file: name is required")
			}
			path, _ := params["path"].(string)
			if path == "" {
				return nil, errors.New("read_skill_file: path is required")
			}
			body, err := idx.LoadSkillFile(name, path)
			if err != nil {
				return nil, err
			}
			return body, nil
		},
	})
}

// ExecuteSkillScriptOptions 脚本执行可选配置（任务 15.1）；nil 或零值使用默认（仅 .sh，30 秒超时）。
type ExecuteSkillScriptOptions struct {
	// AllowedExtensions 允许的扩展名白名单，如 [".sh"]；空时默认 [".sh"]。
	AllowedExtensions []string
	// TimeoutSeconds 单次执行超时（秒）；<=0 时默认 30；建议上限 300。
	TimeoutSeconds int
}

func defaultScriptAllowedExtensions(opts *ExecuteSkillScriptOptions) []string {
	if opts != nil && len(opts.AllowedExtensions) > 0 {
		return opts.AllowedExtensions
	}
	return []string{".sh"}
}

// scriptInterpreter 根据脚本扩展名返回解释器命令（.sh -> sh, .py -> python3）。
func scriptInterpreter(ext string) string {
	switch ext {
	case ".py":
		return "python"
	default:
		return "sh"
	}
}

func defaultScriptTimeout(opts *ExecuteSkillScriptOptions) time.Duration {
	sec := 30
	if opts != nil && opts.TimeoutSeconds > 0 {
		sec = opts.TimeoutSeconds
		if sec > 300 {
			sec = 300
		}
	}
	return time.Duration(sec) * time.Second
}

// RegisterExecuteSkillScriptTool 向 Registry 注册执行 Skill 目录下脚本的工具（可选，受 allowScriptExecution 控制）。
// 当 allowScriptExecution 为 false 时，工具执行直接返回错误；为 true 时在 scripts/ 下按 AllowedExtensions 与 TimeoutSeconds 执行（任务 15.2～15.4）。
// opts 可为 nil，此时默认仅 .sh、30 秒超时。
func RegisterExecuteSkillScriptTool(reg *Registry, idx *skills.Index, allowScriptExecution bool, opts *ExecuteSkillScriptOptions) error {
	if reg == nil {
		return errors.New("execute_skill_script: registry is nil")
	}
	if idx == nil {
		return errors.New("execute_skill_script: index is nil")
	}
	allow := allowScriptExecution
	allowedExt := defaultScriptAllowedExtensions(opts)
	timeout := defaultScriptTimeout(opts)
	return reg.Register(Tool{
		Name:        "execute_skill_script",
		Description: "Execute a script bundled with a Skill (e.g. scripts/run.sh). Only available when script execution is enabled in config; path must be under the skill directory, typically under scripts/.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Skill name (kebab-case).",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Relative path to script under the skill directory, e.g. scripts/run.sh.",
				},
			},
			"required": []string{"name", "path"},
		},
		Execute: func(ctx context.Context, params map[string]any) (any, error) {
			if !allow {
				return nil, errors.New("execute_skill_script: script execution is disabled (set skills.allow_script_execution to enable)")
			}
			name, _ := params["name"].(string)
			if name == "" {
				return nil, errors.New("execute_skill_script: name is required")
			}
			relPath, _ := params["path"].(string)
			if relPath == "" {
				return nil, errors.New("execute_skill_script: path is required")
			}
			meta, ok := idx.GetByName(name)
			if !ok {
				return nil, fmt.Errorf("execute_skill_script: skill not found: %s", name)
			}
			if meta.Path == "" {
				return nil, fmt.Errorf("execute_skill_script: skill path is empty for %s", name)
			}
			skillRoot := filepath.Dir(meta.Path)
			cleaned := filepath.Clean(relPath)
			if cleaned == ".." || strings.HasPrefix(cleaned, "..") {
				return nil, errors.New("execute_skill_script: path must be under skill directory")
			}
			fullPath := filepath.Join(skillRoot, cleaned)
			absRoot, err := filepath.Abs(skillRoot)
			if err != nil {
				return nil, err
			}
			absFull, err := filepath.Abs(fullPath)
			if err != nil {
				return nil, err
			}
			if absFull != absRoot && !strings.HasPrefix(absFull, absRoot+string(filepath.Separator)) {
				return nil, errors.New("execute_skill_script: path must be under skill directory")
			}
			if !strings.HasPrefix(cleaned, "scripts") {
				return nil, errors.New("execute_skill_script: only scripts under scripts/ are allowed")
			}
			ext := filepath.Ext(cleaned)
			allowed := false
			for _, e := range allowedExt {
				if e == ext {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, fmt.Errorf("execute_skill_script: extension %q not in allowed list %v", ext, allowedExt)
			}
			if _, err := os.Stat(fullPath); err != nil {
				return nil, fmt.Errorf("execute_skill_script: script file: %w", err)
			}
			interpreter := scriptInterpreter(ext)
			runCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			cmd := exec.CommandContext(runCtx, interpreter, fullPath)
			cmd.Dir = skillRoot
			out, err := cmd.CombinedOutput()
			if err != nil {
				return string(out) + "\n" + err.Error(), nil
			}
			return string(out), nil
		},
	})
}
