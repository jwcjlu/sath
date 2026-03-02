package datasource

import (
	"context"
	"fmt"
	"strings"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
)

// ESClientProvider 由可提供 Elasticsearch 客户端的数据源实现，供 metadata 与 executor 使用。
type ESClientProvider interface {
	ESClient() *elasticsearch.Client
}

// esDataSource 实现 DataSource 与 ESClientProvider。
type esDataSource struct {
	id     string
	client *elasticsearch.Client
}

func (e *esDataSource) ID() string { return e.id }

func (e *esDataSource) Ping(ctx context.Context) error {
	res, err := e.client.Ping(e.client.Ping.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("elasticsearch ping: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("elasticsearch ping: %s", res.String())
	}
	return nil
}

func (e *esDataSource) Close() error {
	// go-elasticsearch v8 Client 无 Close，连接由 http.Client 管理，此处无操作
	return nil
}

func (e *esDataSource) ESClient() *elasticsearch.Client { return e.client }

// NewElasticsearchDataSource 根据 Config 创建 Elasticsearch 数据源。
// 使用 DSN 作为完整 URL（如 http://localhost:9200），若为空则用 Host:Port（默认 9200）。
// 可选 User/Password 用于 Basic 认证。
func NewElasticsearchDataSource(cfg Config) (*esDataSource, error) {
	if cfg.ID == "" {
		return nil, fmt.Errorf("elasticsearch datasource: missing id")
	}
	addr := cfg.DSN
	if addr == "" {
		host := cfg.Host
		if host == "" {
			host = "localhost"
		}
		port := cfg.Port
		if port == 0 {
			port = 9200
		}
		addr = fmt.Sprintf("http://%s:%d", host, port)
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}

	clientCfg := elasticsearch.Config{
		Addresses: []string{addr},
	}
	if cfg.User != "" {
		clientCfg.Username = cfg.User
		clientCfg.Password = cfg.Password
	}

	client, err := elasticsearch.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch datasource: new client: %w", err)
	}

	return &esDataSource{id: cfg.ID, client: client}, nil
}

// RegisterElasticsearch 在 Registry 上注册 "elasticsearch" 类型的数据源工厂。
func RegisterElasticsearch(r *Registry) {
	if r == nil {
		return
	}
	r.RegisterType("elasticsearch", func(cfg Config) (DataSource, error) {
		return NewElasticsearchDataSource(cfg)
	})
}
