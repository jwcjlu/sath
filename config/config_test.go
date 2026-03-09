package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
model: openai/gpt-4o
max_history: 5
middlewares:
  - logging
  - metrics
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.ModelName != "openai/gpt-4o" {
		t.Fatalf("expected model openai/gpt-4o, got %s", cfg.ModelName)
	}
	if cfg.MaxHistory != 5 {
		t.Fatalf("expected max_history 5, got %d", cfg.MaxHistory)
	}
	if len(cfg.Middlewares) != 2 || cfg.Middlewares[0] != "logging" {
		t.Fatalf("unexpected middlewares: %#v", cfg.Middlewares)
	}
}

func TestLoad_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"model":"openai/gpt-3.5-turbo","max_history":8}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.ModelName != "openai/gpt-3.5-turbo" || cfg.MaxHistory != 8 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	cfg := Config{ModelName: "openai/gpt-4", MaxHistory: 5}
	os.Setenv("OPENAI_MODEL", "openai/gpt-3.5-turbo")
	os.Setenv("AGENT_MAX_HISTORY", "20")
	defer func() {
		os.Unsetenv("OPENAI_MODEL")
		os.Unsetenv("AGENT_MAX_HISTORY")
	}()
	ApplyEnvOverrides(&cfg)
	if cfg.ModelName != "openai/gpt-3.5-turbo" || cfg.MaxHistory != 20 {
		t.Fatalf("expected overrides applied: %#v", cfg)
	}
}

func TestLoadWithEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("model: openai/gpt-4\nmax_history: 3"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Setenv("AGENT_MAX_HISTORY", "7")
	defer os.Unsetenv("AGENT_MAX_HISTORY")
	cfg, err := LoadWithEnv(path)
	if err != nil {
		t.Fatalf("LoadWithEnv: %v", err)
	}
	if cfg.MaxHistory != 7 {
		t.Fatalf("expected env override max_history 7, got %d", cfg.MaxHistory)
	}
}

func TestLoadForEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.dev.yaml")
	if err := os.WriteFile(path, []byte("model: openai/gpt-4o\nmax_history: 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadForEnv("dev", dir)
	if err != nil {
		t.Fatalf("LoadForEnv: %v", err)
	}
	if cfg.ModelName != "openai/gpt-4o" || cfg.MaxHistory != 2 {
		t.Fatalf("unexpected: %#v", cfg)
	}
}

// TestLoad_Skills 验证 Skills 配置可正确解析（任务 1 验收）。
func TestLoad_Skills(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
model: openai/gpt-4o
max_history: 10
skills:
  skills_dirs: [skills, skills.d]
  enabled_skills: [a, b]
  disabled_skills: [c]
  allow_script_execution: true
  script_allowed_extensions: [.sh, .py]
  script_timeout_seconds: 60
  mcp_servers:
    - id: mcp-k8s
      endpoint: http://localhost:8080/mcp
      backend: metoro
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Skills.Dirs) != 2 || cfg.Skills.Dirs[0] != "skills" {
		t.Fatalf("unexpected skills_dirs: %v", cfg.Skills.Dirs)
	}
	if len(cfg.Skills.EnabledSkills) != 2 || cfg.Skills.EnabledSkills[0] != "a" {
		t.Fatalf("unexpected enabled_skills: %v", cfg.Skills.EnabledSkills)
	}
	if len(cfg.Skills.DisabledSkills) != 1 || cfg.Skills.DisabledSkills[0] != "c" {
		t.Fatalf("unexpected disabled_skills: %v", cfg.Skills.DisabledSkills)
	}
	if !cfg.Skills.AllowScriptExecution {
		t.Fatal("expected allow_script_execution true")
	}
	if len(cfg.Skills.ScriptAllowedExtensions) != 2 || cfg.Skills.ScriptAllowedExtensions[0] != ".sh" {
		t.Fatalf("unexpected script_allowed_extensions: %v", cfg.Skills.ScriptAllowedExtensions)
	}
	if cfg.Skills.ScriptTimeoutSeconds != 60 {
		t.Fatalf("expected script_timeout_seconds 60, got %d", cfg.Skills.ScriptTimeoutSeconds)
	}
	if len(cfg.Skills.MCPServers) != 1 || cfg.Skills.MCPServers[0].ID != "mcp-k8s" {
		t.Fatalf("unexpected mcp_servers: %v", cfg.Skills.MCPServers)
	}
}

// TestLoad_SkillsEmpty 未配置 Skills 时零值不影响（任务 1 验收）。
func TestLoad_SkillsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("model: openai/gpt-4o\nmax_history: 5"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Skills.Dirs != nil || cfg.Skills.EnabledSkills != nil || cfg.Skills.AllowScriptExecution != false {
		t.Fatalf("expected zero value skills when not configured: %#v", cfg.Skills)
	}
}
