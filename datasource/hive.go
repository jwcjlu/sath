package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// hiveDataSource 实现 DataSource，并暴露底层 *sql.DB 供执行器与元数据使用。
// 具体 Hive 驱动（如 HiveServer2/Trino/Presto 等）的注册应由应用在 main 或其他包中通过 database/sql 注册。
type hiveDataSource struct {
	id string
	db *sql.DB
}

func (h *hiveDataSource) ID() string { return h.id }

func (h *hiveDataSource) Ping(ctx context.Context) error {
	return h.db.PingContext(ctx)
}

func (h *hiveDataSource) Close() error {
	return h.db.Close()
}

// DB 返回底层 *sql.DB，供 executor 与 metadata 使用。
func (h *hiveDataSource) DB() *sql.DB {
	return h.db
}

// NewHiveDataSource 根据 Config 打开 Hive 连接并配置连接池。
// DSN 由具体驱动定义；若未提供 DSN，则按 host/port/user 构造一个常见形式（实际仍需驱动支持）。
func NewHiveDataSource(cfg Config) (*hiveDataSource, error) {
	if cfg.ID == "" {
		return nil, fmt.Errorf("hive datasource: missing id")
	}
	dsn := cfg.DSN
	if dsn == "" {
		if cfg.Host == "" {
			return nil, fmt.Errorf("hive datasource: incomplete config for id=%s", cfg.ID)
		}
		host := cfg.Host
		port := cfg.Port
		if port == 0 {
			port = 10000
		}
		// 这里不强绑具体驱动 DSN 语法，仅给出一个常见占位形式；实际可通过 DSN 字段覆盖。
		if cfg.User != "" {
			dsn = fmt.Sprintf("%s:%d/%s?user=%s", host, port, cfg.DBName, cfg.User)
		} else {
			dsn = fmt.Sprintf("%s:%d/%s", host, port, cfg.DBName)
		}
	}

	// 注意：此处 driverName 使用 "hive"，要求上层提前通过 database/sql 注册对应驱动。
	db, err := sql.Open("hive", dsn)
	if err != nil {
		return nil, fmt.Errorf("hive datasource: open: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}

	return &hiveDataSource{
		id: cfg.ID,
		db: db,
	}, nil
}

// RegisterHive 在 Registry 上注册 "hive" 类型的数据源工厂。
func RegisterHive(r *Registry) {
	if r == nil {
		return
	}
	r.RegisterType("hive", func(cfg Config) (DataSource, error) {
		return NewHiveDataSource(cfg)
	})
}
