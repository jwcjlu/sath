package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const initMainGo = `package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sath/agent"
	"github.com/sath/config"
	"github.com/sath/memory"
	"github.com/sath/middleware"
	"github.com/sath/model"
)

func main() {
	ctx := context.Background()
	cfg := config.FromEnv()

	m, err := model.NewOpenAIClient()
	if err != nil {
		fmt.Println("init model error:", err)
		os.Exit(1)
	}

	mem := memory.NewBufferMemory(cfg.MaxHistory)
	core := agent.NewChatAgent(m, mem, agent.WithMaxHistory(cfg.MaxHistory))
	handler := middleware.Chain(
		func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
			return core.Run(ctx, req)
		},
		middleware.RecoveryMiddleware,
		middleware.LoggingMiddleware,
	)

	fmt.Println("Agent started. Type 'exit' to quit.")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "exit" {
			break
		}
		req := &agent.Request{
			Messages: []model.Message{{Role: "user", Content: line}},
		}
		resp, err := handler(ctx, req)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println(resp.Text)
	}
}
`

const initConfigYaml = `# sath Agent 配置示例
model: openai/gpt-3.5-turbo
max_history: 10
middlewares:
  - logging
  - metrics
`

// NewInitCommand 返回 sath init 命令。
func NewInitCommand() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "初始化一个新项目骨架",
		Long:  "在当前目录或指定目录下生成 main.go 与 config.yaml 示例，便于快速开始。",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
			}
			base := dir
			mainPath := filepath.Join(base, "main.go")
			if _, err := os.Stat(mainPath); err == nil {
				return fmt.Errorf("main.go already exists in %q", base)
			}
			if err := os.WriteFile(mainPath, []byte(initMainGo), 0o644); err != nil {
				return err
			}
			configPath := filepath.Join(base, "config.yaml")
			if err := os.WriteFile(configPath, []byte(initConfigYaml), 0o644); err != nil {
				return err
			}
			cmd.Println("Created", mainPath, "and", configPath)
			cmd.Println("Run: go mod init <module> && go get github.com/sath && go run .")
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "输出目录，默认为当前目录")
	return cmd
}
