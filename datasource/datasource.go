package datasource

import "context"

// DataSource 表示可连接、健康检查与关闭的数据源抽象。
// 执行 DSL 由 executor 包负责，本接口仅做连接与存活检查。
type DataSource interface {
	ID() string
	Ping(ctx context.Context) error
	Close() error
}

// Config 为数据源连接配置。敏感字段（如 Password）建议从环境变量或密钥服务读取，不落明文配置。
type Config struct {
	ID               string `json:"id" yaml:"id"`
	Type             string `json:"type" yaml:"type"` // 如 "mysql"
	DSN              string `json:"dsn" yaml:"dsn"`   // 完整 DSN，与 Host/Port/User/Password/DBName 二选一
	Host             string `json:"host" yaml:"host"`
	Port             int    `json:"port" yaml:"port"`
	User             string `json:"user" yaml:"user"`
	Password         string `json:"password" yaml:"password"`
	DBName           string `json:"dbname" yaml:"dbname"`
	MaxOpenConns     int    `json:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns     int    `json:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime  int    `json:"conn_max_lifetime_sec" yaml:"conn_max_lifetime_sec"` // 秒
	ReadOnly         bool   `json:"read_only" yaml:"read_only"`
}
