package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sath/model"
	"github.com/sath/tool"
)

// 本 demo 展示如何通过 OpenAI Function Calling + 本地工具执行，实现一个加法计算示例。
func main() {
	ctx := context.Background()

	m, err := model.NewOpenAIClient()
	if err != nil {
		fmt.Println("init model error:", err)
		return
	}

	reg := tool.NewRegistry()
	if err := tool.RegisterCalculatorTool(reg); err != nil {
		fmt.Println("register tool error:", err)
		return
	}

	// 提示模型：当需要做加法时，使用 calculator_add 工具。
	prompt := "请计算 1.5 + 2.5 的结果，只需调用工具，不要直接给出答案。"
	msgs := []model.Message{
		{Role: "user", Content: prompt},
	}

	gen, err := m.ChatWithTools(ctx, msgs, reg)
	if err != nil {
		fmt.Println("chat with tools error:", err)
		return
	}

	// ChatWithTools 返回的是工具执行结果的 JSON 文本，例如 "4" 或 "3.0"。
	var result any
	if err := json.Unmarshal([]byte(gen.Text), &result); err != nil {
		fmt.Println("tool result:", gen.Text)
		return
	}

	fmt.Println("calculator_add result:", result)
}
