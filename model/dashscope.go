package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

const dashScopeBaseURL = "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"
const defaultDashScopeModel = "qwen-turbo"

// DashScopeClient 阿里云通义千问（DashScope）文本模型适配器，一期仅 Chat/Generate（B.1.1）。
type DashScopeClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewDashScopeClient 从环境变量 DASHSCOPE_API_KEY 创建客户端，可选 DASHSCOPE_MODEL。
func NewDashScopeClient() (*DashScopeClient, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return nil, errors.New("missing DASHSCOPE_API_KEY")
	}
	model := os.Getenv("DASHSCOPE_MODEL")
	if model == "" {
		model = defaultDashScopeModel
	}
	return &DashScopeClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type dashScopeReq struct {
	Model      string          `json:"model"`
	Input      dashScopeInput  `json:"input"`
	Parameters dashScopeParams `json:"parameters"`
}

type dashScopeInput struct {
	Messages []dashScopeMsg `json:"messages"`
}

type dashScopeMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dashScopeParams struct {
	ResultFormat string  `json:"result_format,omitempty"` // "message" | "text"
	MaxTokens    int     `json:"max_tokens,omitempty"`
	Temperature  float64 `json:"temperature,omitempty"`
}

type dashScopeResp struct {
	Output dashScopeOutput `json:"output"`
	Usage  *dashScopeUsage `json:"usage"`
}

type dashScopeOutput struct {
	Text string `json:"text"`
}

type dashScopeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (c *DashScopeClient) Generate(ctx context.Context, prompt string, opts ...Option) (*Generation, error) {
	msgs := []Message{{Role: "user", Content: prompt}}
	return c.Chat(ctx, msgs, opts...)
}

func (c *DashScopeClient) Chat(ctx context.Context, messages []Message, opts ...Option) (*Generation, error) {
	if len(messages) == 0 {
		return nil, errors.New("messages is empty")
	}
	callCfg := ApplyOptions(opts...)
	modelName := c.model
	if callCfg.ModelName != "" {
		modelName = callCfg.ModelName
	}
	dsMsgs := make([]dashScopeMsg, 0, len(messages))
	for _, m := range messages {
		dsMsgs = append(dsMsgs, dashScopeMsg{Role: m.Role, Content: m.Content})
	}
	params := dashScopeParams{ResultFormat: "message"}
	if callCfg.MaxTokens > 0 {
		params.MaxTokens = callCfg.MaxTokens
	}
	if callCfg.Temperature > 0 {
		params.Temperature = float64(callCfg.Temperature)
	}
	body := dashScopeReq{
		Model:      modelName,
		Input:      dashScopeInput{Messages: dsMsgs},
		Parameters: params,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dashScopeBaseURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dashscope api: %s", resp.Status)
	}
	var out dashScopeResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	gen := &Generation{Text: out.Output.Text}
	if out.Usage != nil {
		gen.TokenUsage = &TokenUsage{
			InputTokens:  out.Usage.InputTokens,
			OutputTokens: out.Usage.OutputTokens,
		}
	}
	return gen, nil
}

func (c *DashScopeClient) Embed(ctx context.Context, texts []string, opts ...Option) ([]Embedding, error) {
	return nil, errors.New("dashscope: Embed not implemented")
}
