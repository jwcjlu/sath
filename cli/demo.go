package cli

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
	"github.com/spf13/cobra"
)

// NewDemoCommand 返回 sath demo 命令，运行内置对话示例。
func NewDemoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "运行内置对话 Agent 示例",
		Long:  "启动一个 REPL，使用默认配置的对话 Agent。需设置 OPENAI_API_KEY。",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg := config.FromEnv()

			m, err := model.NewOpenAIClient()
			if err != nil {
				return fmt.Errorf("init model: %w", err)
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

			cmd.Println("AI Agent demo started. Type 'exit' to quit.")
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Print("> ")
				line, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if line == "exit" {
					return nil
				}
				req := &agent.Request{
					Messages: []model.Message{{Role: "user", Content: line}},
				}
				resp, err := handler(ctx, req)
				if err != nil {
					fmt.Println("agent error:", err)
					continue
				}
				fmt.Println(resp.Text)
			}
		},
	}
	return cmd
}
