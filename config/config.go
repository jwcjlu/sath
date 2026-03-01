package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sath/datasource"
	yaml "go.yaml.in/yaml/v2"
)

// Config 保存框架在 V0.2 阶段的核心配置。
type Config struct {
	// 默认模型标识，如 "openai/gpt-4o"。
	ModelName string `json:"model" yaml:"model"`
	// 会话短期记忆窗口大小。
	MaxHistory int `json:"max_history" yaml:"max_history"`
	// 启用的中间件名称列表，例如 ["logging","metrics","tracing","cache"]。
	Middlewares []string `json:"middlewares" yaml:"middlewares"`

	// DataSources 定义可用于数据查询 Agent 的数据源列表（可为空）。
	DataSources []datasource.Config `json:"data_sources" yaml:"data_sources"`
	// DefaultDatasourceID 为数据查询 Agent 默认使用的数据源 ID（可被请求覆盖）。
	DefaultDatasourceID string `json:"default_datasource_id" yaml:"default_datasource_id"`
	// DataAllowWrite 控制数据查询 Agent 是否允许写/改；为 false 时仅启用只读工具。
	DataAllowWrite bool `json:"data_allow_write" yaml:"data_allow_write"`
}

// FromEnv 从环境变量加载核心配置。
// 目前支持：
// - OPENAI_MODEL: 默认模型名称
// - AGENT_MAX_HISTORY: 会话记忆窗口大小
func FromEnv() Config {
	cfg := Config{
		ModelName:   os.Getenv("OPENAI_MODEL"),
		MaxHistory:  10,
		Middlewares: []string{},
	}
	if v := os.Getenv("AGENT_MAX_HISTORY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxHistory = n
		}
	}
	return cfg
}

// Load 从给定路径加载配置文件，支持 YAML（.yaml/.yml）与 JSON。
// 若文件不存在或解析失败，将返回错误。
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	ext := filepath.Ext(path)
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	default:
		return cfg, errors.New("unsupported config file extension: " + ext)
	}

	// 合理默认值补齐。
	if cfg.MaxHistory <= 0 {
		cfg.MaxHistory = 10
	}
	return cfg, nil
}

// ApplyEnvOverrides 用环境变量覆盖 cfg 中对应字段（未设置的环境变量不覆盖）。
// 用于在 Load 之后叠加环境变量，实现 B.2.1 环境变量覆盖。
func ApplyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	if v := os.Getenv("OPENAI_MODEL"); v != "" {
		cfg.ModelName = v
	}
	if v := os.Getenv("AGENT_MAX_HISTORY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxHistory = n
		}
	}
	// 可选：AGENT_MIDDLEWARES 逗号分隔，覆盖中间件列表
	if v := os.Getenv("AGENT_MIDDLEWARES"); v != "" {
		var list []string
		for _, s := range splitAndTrim(v, ",") {
			if s != "" {
				list = append(list, s)
			}
		}
		if len(list) > 0 {
			cfg.Middlewares = list
		}
	}
	if v := os.Getenv("DEFAULT_DATASOURCE_ID"); v != "" {
		cfg.DefaultDatasourceID = v
	}
	if v := os.Getenv("DATA_ALLOW_WRITE"); v != "" {
		lower := strings.ToLower(strings.TrimSpace(v))
		cfg.DataAllowWrite = lower == "1" || lower == "true" || lower == "yes"
	}
}

func splitAndTrim(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// LoadWithEnv 加载配置文件并用环境变量覆盖部分字段（B.2.1）。
func LoadWithEnv(path string) (Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return cfg, err
	}
	ApplyEnvOverrides(&cfg)
	return cfg, nil
}

// LoadForEnv 按环境名加载多配置文件（B.2.2）。
// 若 env 为空则使用 "dev"。路径规则：dir 为目录时加载 dir/config.<env>.<ext>，否则将 dir 视为前缀，加载 <dir>.<env>.<ext>。
// 例如 LoadForEnv("dev", "config") 尝试 config.dev.yaml / config.dev.yml / config.dev.json。
func LoadForEnv(env, dir string) (Config, error) {
	if env == "" {
		env = "dev"
	}
	exts := []string{".yaml", ".yml", ".json"}
	for _, ext := range exts {
		path := filepath.Join(dir, "config."+env+ext)
		cfg, err := Load(path)
		if err == nil {
			ApplyEnvOverrides(&cfg)
			return cfg, nil
		}
		if !os.IsNotExist(err) {
			return cfg, err
		}
	}
	return Config{}, errors.New("no config file found for env: " + env)
}
