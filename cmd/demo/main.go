package main

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
		return
	}

	mem := memory.NewBufferMemory(cfg.MaxHistory)
	coreAgent := agent.NewChatAgent(m, mem, agent.WithMaxHistory(cfg.MaxHistory))

	handler := middleware.Chain(
		func(ctx context.Context, req *agent.Request) (*agent.Response, error) {
			return coreAgent.Run(ctx, req)
		},
		middleware.RecoveryMiddleware,
		middleware.LoggingMiddleware,
	)

	fmt.Println("AI Agent demo started. Type 'exit' to quit.")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("read error:", err)
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "exit" {
			return
		}

		req := &agent.Request{
			Messages: []model.Message{
				{Role: "user", Content: line},
			},
		}
		resp, err := handler(ctx, req)
		if err != nil {
			fmt.Println("agent error:", err)
			continue
		}
		fmt.Println(resp.Text)
	}
}
