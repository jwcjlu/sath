package model

import (
	"os"
	"testing"
)

func TestParseModelIdentifier(t *testing.T) {
	tests := []struct {
		in       string
		provider string
		model    string
	}{
		{"", "", ""},
		{"openai", "openai", ""},
		{"OPENAI", "openai", ""},
		{"openai/gpt-4o", "openai", "gpt-4o"},
		{" openai / gpt-3.5-turbo ", "openai", "gpt-3.5-turbo"},
	}

	for _, tt := range tests {
		p, m := parseModelIdentifier(tt.in)
		if p != tt.provider || m != tt.model {
			t.Fatalf("parseModelIdentifier(%q) = (%q,%q), want (%q,%q)", tt.in, p, m, tt.provider, tt.model)
		}
	}
}

func TestNewFromIdentifier_OpenAI(t *testing.T) {
	// 使用假 key，只验证构造逻辑，不发真实请求。
	if err := os.Setenv("OPENAI_API_KEY", "test-key"); err != nil {
		t.Fatalf("Setenv error: %v", err)
	}
	defer os.Unsetenv("OPENAI_API_KEY")

	os.Unsetenv("OPENAI_MODEL")

	m, err := NewFromIdentifier("openai/gpt-4o")
	if err != nil {
		t.Fatalf("NewFromIdentifier error: %v", err)
	}

	cli, ok := m.(*OpenAIClient)
	if !ok {
		t.Fatalf("expected *OpenAIClient, got %T", m)
	}
	if cli.model != "gpt-4o" {
		t.Fatalf("expected model gpt-4o, got %s", cli.model)
	}
}

func TestNewFromIdentifier_Ollama(t *testing.T) {
	// 配置环境变量，验证 Ollama 分支能够被构造。
	if err := os.Setenv("OLLAMA_MODEL", "llama3"); err != nil {
		t.Fatalf("Setenv error: %v", err)
	}
	defer os.Unsetenv("OLLAMA_MODEL")

	m, err := NewFromIdentifier("ollama/llama3")
	if err != nil {
		t.Fatalf("NewFromIdentifier error: %v", err)
	}
	if _, ok := m.(*OllamaClient); !ok {
		t.Fatalf("expected *OllamaClient, got %T", m)
	}
}
