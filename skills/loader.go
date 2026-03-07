package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadSkillBody 根据 Skill 名称加载完整的 SKILL.md 正文内容。
func (idx *Index) LoadSkillBody(name string) (string, error) {
	if idx == nil {
		return "", fmt.Errorf("skills: index is nil")
	}
	if name == "" {
		return "", fmt.Errorf("skills: name is empty")
	}
	meta, ok := idx.GetByName(name)
	if !ok {
		return "", fmt.Errorf("skills: skill not found: %s", name)
	}
	if meta.Path == "" {
		return "", fmt.Errorf("skills: skill path is empty for %s", name)
	}
	data, err := os.ReadFile(meta.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// LoadSkillFile 根据 Skill 名称与相对路径加载该 Skill 目录下的捆绑文件（如 docs/、assets/、scripts/ 下的文件）。
// relativePath 相对于 Skill 根目录，例如 "docs/advanced.md"、"assets/template.json"。
// 禁止通过 ".." 访问 Skill 目录外的路径，否则返回错误。
func (idx *Index) LoadSkillFile(name, relativePath string) (string, error) {
	if idx == nil {
		return "", fmt.Errorf("skills: index is nil")
	}
	if name == "" {
		return "", fmt.Errorf("skills: name is empty")
	}
	if relativePath == "" {
		return "", fmt.Errorf("skills: relative path is empty")
	}
	meta, ok := idx.GetByName(name)
	if !ok {
		return "", fmt.Errorf("skills: skill not found: %s", name)
	}
	if meta.Path == "" {
		return "", fmt.Errorf("skills: skill path is empty for %s", name)
	}
	skillRoot := filepath.Dir(meta.Path)
	// 规范化相对路径，并确保结果仍在 skillRoot 之下，防止 ".." 逃逸
	cleaned := filepath.Clean(relativePath)
	if cleaned == ".." || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("skills: path must be under skill directory: %s", relativePath)
	}
	fullPath := filepath.Join(skillRoot, cleaned)
	absRoot, err := filepath.Abs(skillRoot)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	if absFull != absRoot && !strings.HasPrefix(absFull, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("skills: path must be under skill directory: %s", relativePath)
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
