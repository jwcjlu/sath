package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCommand 返回 sath 的根命令。
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "sath",
		Short: "sath AI Agent 框架命令行工具",
		Long:  "用于初始化项目、运行示例与启动本地 Agent 服务的 CLI。",
	}
	root.AddCommand(NewInitCommand())
	root.AddCommand(NewDemoCommand())
	root.AddCommand(NewServeCommand())
	return root
}
