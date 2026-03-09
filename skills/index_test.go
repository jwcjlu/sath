package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewIndex_EmptyDirs(t *testing.T) {
	idx, err := NewIndex(nil, nil, nil)
	if err != nil {
		t.Fatalf("NewIndex(nil,nil,nil): %v", err)
	}
	if idx == nil {
		t.Fatal("expected non-nil index")
	}
	if n := len(idx.All()); n != 0 {
		t.Fatalf("expected 0 skills, got %d", n)
	}
}

func TestNewIndex_OneSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: my-skill
description: A test skill.
tags: [test]
scope: [dataquery]
allowed_tools: [execute_write]
---
# Body
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	idx, err := NewIndex([]string{dir}, nil, nil)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	all := idx.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(all))
	}
	if all[0].Name != "my-skill" || all[0].Description != "A test skill." {
		t.Fatalf("unexpected meta: %+v", all[0])
	}
	if all[0].Path != path {
		t.Fatalf("expected path %q, got %q", path, all[0].Path)
	}

	// scope & allowed_tools
	if len(all[0].Scopes) != 1 || all[0].Scopes[0] != "dataquery" {
		t.Fatalf("unexpected scopes: %+v", all[0].Scopes)
	}
	if len(all[0].AllowedTools) != 1 || all[0].AllowedTools[0] != "execute_write" {
		t.Fatalf("unexpected allowed_tools: %+v", all[0].AllowedTools)
	}

	body, err := idx.LoadSkillBody("my-skill")
	if err != nil {
		t.Fatalf("LoadSkillBody: %v", err)
	}
	if body == "" || body != content {
		t.Fatalf("LoadSkillBody: expected full file content, got len=%d", len(body))
	}
}

func TestNewIndex_EnabledDisabled(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		skillDir := filepath.Join(dir, name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		path := filepath.Join(skillDir, "SKILL.md")
		content := "---\nname: " + name + "\ndescription: " + name + "\n---\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	idx, err := NewIndex([]string{dir}, []string{"a", "c"}, nil)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	all := idx.All()
	if len(all) != 2 {
		t.Fatalf("enabled [a,c]: expected 2 skills, got %d", len(all))
	}
	names := make(map[string]bool)
	for _, m := range all {
		names[m.Name] = true
	}
	if !names["a"] || !names["c"] || names["b"] {
		t.Fatalf("unexpected skills: %v", names)
	}

	idx2, err := NewIndex([]string{dir}, nil, []string{"b"})
	if err != nil {
		t.Fatalf("NewIndex disabled: %v", err)
	}
	all2 := idx2.All()
	if len(all2) != 2 {
		t.Fatalf("disabled [b]: expected 2 skills, got %d", len(all2))
	}
}

func TestLoadSkillBody_NotFound(t *testing.T) {
	idx, _ := NewIndex(nil, nil, nil)
	_, err := idx.LoadSkillBody("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
}

func TestNewIndex_MCPServersAndMCPServerIDsFromSkills(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"skill-a", "skill-b"} {
		skillDir := filepath.Join(dir, name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		path := filepath.Join(skillDir, "SKILL.md")
		var content string
		if name == "skill-a" {
			content = "---\nname: skill-a\ndescription: a\nscope: [chat]\nmcp_servers: [fs, github]\nmcp_tools: [read_file]\n---\n"
		} else {
			content = "---\nname: skill-b\ndescription: b\nscope: [dataquery]\nmcp_servers: [github]\n---\n"
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	idx, err := NewIndex([]string{dir}, nil, nil)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	all := idx.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(all))
	}
	var skillA, skillB SkillMeta
	for _, m := range all {
		if m.Name == "skill-a" {
			skillA = m
		} else {
			skillB = m
		}
	}
	if len(skillA.MCPServers) != 2 || skillA.MCPServers[0] != "fs" || skillA.MCPServers[1] != "github" {
		t.Fatalf("unexpected skill-a MCPServers: %+v", skillA.MCPServers)
	}
	if len(skillA.MCPTools) != 1 || skillA.MCPTools[0] != "read_file" {
		t.Fatalf("unexpected skill-a MCPTools: %+v", skillA.MCPTools)
	}
	if len(skillB.MCPServers) != 1 || skillB.MCPServers[0] != "github" {
		t.Fatalf("unexpected skill-b MCPServers: %+v", skillB.MCPServers)
	}
	idsChat := idx.MCPServerIDsFromSkills("chat")
	if len(idsChat) != 2 {
		t.Fatalf("expected 2 MCP server IDs for chat scope, got %v", idsChat)
	}
	idsDQ := idx.MCPServerIDsFromSkills("dataquery")
	if len(idsDQ) != 1 || idsDQ[0] != "github" {
		t.Fatalf("expected [github] for dataquery scope, got %v", idsDQ)
	}
}

func TestFilterByScopeAndAnyAllowsTool(t *testing.T) {
	dir := t.TempDir()
	// global skill（无 scope）
	writeSkill := filepath.Join(dir, "global-write")
	_ = os.MkdirAll(writeSkill, 0o755)
	_ = os.WriteFile(filepath.Join(writeSkill, "SKILL.md"), []byte(`---
name: global-write
description: global
allowed_tools: [execute_write]
---
`), 0o644)

	// chat-only skill
	chatSkill := filepath.Join(dir, "chat")
	_ = os.MkdirAll(chatSkill, 0o755)
	_ = os.WriteFile(filepath.Join(chatSkill, "SKILL.md"), []byte(`---
name: chat-only
description: chat
scope: [chat]
---
`), 0o644)

	idx, err := NewIndex([]string{dir}, nil, nil)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}

	dataSkills := idx.FilterByScope("dataquery")
	if len(dataSkills) == 0 {
		t.Fatalf("expected at least one skill visible for dataquery, got 0")
	}
	if !idx.AnyAllowsTool("dataquery", "execute_write") {
		t.Fatalf("expected execute_write to be allowed for dataquery via global skill")
	}
	if !idx.AnyAllowsTool("chat", "execute_write") {
		t.Fatalf("expected execute_write to be allowed for chat via global skill")
	}
	if idx.AnyAllowsTool("dataquery", "nonexistent") {
		t.Fatalf("nonexistent tool should not be allowed")
	}
}
