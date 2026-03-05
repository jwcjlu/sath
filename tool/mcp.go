package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	markclient "github.com/mark3labs/mcp-go/client"
	marktransport "github.com/mark3labs/mcp-go/client/transport"
	markmcp "github.com/mark3labs/mcp-go/mcp"
	mcpmetoro "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
	"github.com/sath/obs"
)

// mcpClient 抽象 MCP 客户端能力，便于在 mcpTool 中解耦具体实现。
type mcpClient interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, args map[string]any) (string, error)
	Close(ctx context.Context) error
}

// mcpTool 仅依赖 mcpClient 接口，不关心底层使用哪个库。
type mcpTool struct {
	client mcpClient
}

type McpConfig struct {
	Endpoint string
	Id       string
	Backend  string
	mcpTool  *mcpTool
}

// NewMcpTool 根据配置创建底层 MCP 客户端，并包装为 mcpTool。
func NewMcpTool(cfg *McpConfig) (*mcpTool, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if backend == "" {
		backend = "metoro"
	}
	var cli mcpClient
	var err error
	switch backend {
	case "mark3labs":
		cli, err = newMark3labsClient(cfg.Endpoint)
	default:
		cli, err = newMetoroClient(cfg.Endpoint)
	}
	if err != nil {
		return nil, err
	}
	return &mcpTool{client: cli}, nil
}

// Initialize 初始化 MCP 会话
func (c *mcpTool) Initialize(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("mcp: client is nil")
	}
	return c.client.Initialize(ctx)
}

// ListTools 获取工具列表
func (c *mcpTool) ListTools(ctx context.Context) ([]Tool, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("mcp: client is nil")
	}
	return c.client.ListTools(ctx)
}

// Close 关闭客户端
func (c *mcpTool) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close(context.Background())
}

func RegisterMcpTool(r *Registry, cfg *McpConfig, opts ...*RegisterToolOptions) {
	mcpTool, err := NewMcpTool(cfg)
	if err != nil {
		return
	}
	err = mcpTool.Initialize(context.Background())
	if err != nil {
		return
	}
	tools, err := mcpTool.ListTools(context.Background())
	if err != nil {
		return
	}
	cfg.mcpTool = mcpTool
	for _, tool := range tools {
		tool.Execute = buildMcpExecute(cfg, tool.Name)
		r.Register(tool)
	}
}

func buildMcpExecute(cfg *McpConfig, toolName string) ExecuteFunc {
	return func(ctx context.Context, params map[string]any) (any, error) {
		start := time.Now()
		status := "ok"
		defer func() {
			obs.ObserveDataQueryTool(toolName, status, time.Since(start))
		}()

		if cfg == nil {
			status = "error"
			return nil, errors.New("mcp: not configured ")
		}

		if cfg.mcpTool == nil || cfg.mcpTool.client == nil {
			status = "error"
			return nil, fmt.Errorf("mcp: client not initialized")
		}

		responseText, err := cfg.mcpTool.client.CallTool(ctx, toolName, params)
		if err != nil {
			status = "error"
			return "", fmt.Errorf("failed to call MCP tool %s: %w", toolName, err)
		}
		return responseText, nil
	}
}

// metoroClientAdapter 基于 github.com/metoro-io/mcp-golang 的客户端实现 mcpClient。
type metoroClientAdapter struct {
	cli *mcpmetoro.Client
}

func newMetoroClient(endpoint string) (mcpClient, error) {

	transport := mcphttp.NewHTTPClientTransport(endpoint)
	transport.WithHeader("Accept", "application/json, text/event-stream")

	// 创建 MCP 客户端
	cli := mcpmetoro.NewClient(transport)

	adapter := &metoroClientAdapter{
		cli: cli,
	}

	return adapter, nil
	/*httpTransport := mcphttp.NewHTTPClientTransport("/mcp")
	httpTransport.WithBaseURL(endpoint)
		cli := mcpmetoro.NewClient(httpTransport)
		return &metoroClientAdapter{cli: cli}, nil*/
}

func (a *metoroClientAdapter) Initialize(ctx context.Context) error {
	if a.cli == nil {
		return fmt.Errorf("metoro client is nil")
	}
	_, err := a.cli.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize metoro MCP client: %w", err)
	}
	return nil
}

func (a *metoroClientAdapter) ListTools(ctx context.Context) ([]Tool, error) {
	if a.cli == nil {
		return nil, fmt.Errorf("metoro client is nil")
	}
	res, err := a.cli.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP tools (metoro): %w", err)
	}
	out := make([]Tool, 0, len(res.Tools))
	for _, t := range res.Tools {
		out = append(out, Tool{
			Name:        t.Name,
			Description: derefString(t.Description),
			Parameters:  t.InputSchema,
		})
	}
	return out, nil
}

func (a *metoroClientAdapter) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	if a.cli == nil {
		return "", fmt.Errorf("metoro client is nil")
	}
	resp, err := a.cli.CallTool(ctx, name, args)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Content) == 0 {
		return "工具执行完成", nil
	}
	var b strings.Builder
	for _, c := range resp.Content {
		if c.TextContent != nil {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(c.TextContent.Text)
			continue
		}
		if data, err := json.Marshal(c); err == nil {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(string(data))
		}
	}
	if b.Len() == 0 {
		return "工具执行完成", nil
	}
	return b.String(), nil
}

func (a *metoroClientAdapter) Close(ctx context.Context) error {
	// 当前 mcp-golang 客户端未暴露 Close 方法，这里直接返回 nil。
	return nil
}

// mark3labsClientAdapter 基于 github.com/mark3labs/mcp-go 的客户端实现 mcpClient。
type mark3labsClientAdapter struct {
	cli *markclient.Client
}

func newMark3labsClient(endpoint string) (mcpClient, error) {
	httpTransport, err := marktransport.NewStreamableHTTP(
		endpoint,
		marktransport.WithHTTPHeaders(map[string]string{
			"Accept": "application/json, text/event-stream",
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mark3labs HTTP transport: %w", err)
	}
	cli := markclient.NewClient(httpTransport)
	if err := cli.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start mark3labs MCP client: %w", err)
	}
	return &mark3labsClientAdapter{cli: cli}, nil
}

func (a *mark3labsClientAdapter) Initialize(ctx context.Context) error {
	if a.cli == nil {
		return fmt.Errorf("mark3labs client is nil")
	}
	// 初始化 MCP 会话（必须调用，否则客户端未初始化）
	initRequest := markmcp.InitializeRequest{
		Params: markmcp.InitializeParams{
			ProtocolVersion: markmcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    markmcp.ClientCapabilities{},
			ClientInfo: markmcp.Implementation{
				Name:    "sath",
				Version: "1.0.0",
			},
		},
	}
	if _, err := a.cli.Initialize(ctx, initRequest); err != nil {
		return fmt.Errorf("failed to initialize mark3labs MCP client: %w", err)
	}
	return nil
}

func (a *mark3labsClientAdapter) ListTools(ctx context.Context) ([]Tool, error) {
	if a.cli == nil {
		return nil, fmt.Errorf("mark3labs client is nil")
	}
	res, err := a.cli.ListTools(ctx, markmcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP tools (mark3labs): %w", err)
	}
	out := make([]Tool, 0, len(res.Tools))
	for _, t := range res.Tools {
		out = append(out, Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	return out, nil
}

func (a *mark3labsClientAdapter) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	if a.cli == nil {
		return "", fmt.Errorf("mark3labs client is nil")
	}
	req := markmcp.CallToolRequest{
		Params: markmcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
	resp, err := a.cli.CallTool(ctx, req)
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Content) == 0 {
		return "工具执行完成", nil
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return "工具执行完成", nil
	}
	return string(data), nil
}

func (a *mark3labsClientAdapter) Close(ctx context.Context) error {
	if a.cli == nil {
		return nil
	}
	return a.cli.Close()
}

// derefString 将 *string 转为 string，nil 时返回空串。
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
