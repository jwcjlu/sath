package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSkillFile(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	docsDir := filepath.Join(skillDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	docPath := filepath.Join(docsDir, "extra.md")
	content := "---\nname: my-skill\ndescription: test\n---\n"
	if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(docPath, []byte("# Extra doc"), 0o644); err != nil {
		t.Fatalf("write docs/extra.md: %v", err)
	}

	idx, err := NewIndex([]string{dir}, nil, nil)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}

	body, err := idx.LoadSkillFile("my-skill", "docs/extra.md")
	if err != nil {
		t.Fatalf("LoadSkillFile: %v", err)
	}
	if body != "# Extra doc" {
		t.Fatalf("unexpected content: %q", body)
	}

	// 禁止 .. 逃逸
	_, err = idx.LoadSkillFile("my-skill", "../other/skip")
	if err == nil {
		t.Fatal("expected error for .. path")
	}
}
