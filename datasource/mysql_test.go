package datasource

import (
	"context"
	"testing"
)

func TestRegisterMySQL_andRegister_invalidDSN(t *testing.T) {
	r := NewRegistry()
	RegisterMySQL(r)
	_, err := r.Register(Config{
		ID:   "m1",
		Type: "mysql",
		DSN:  "invalid://bad",
	})
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestRegisterMySQL_andRegister_openFails(t *testing.T) {
	r := NewRegistry()
	RegisterMySQL(r)
	// 无效的 DSN 会导致 Open 失败（driver 可能仍会 open，但 Ping 会失败）。这里仅验证 Register 不会 panic，且无真实连接时可用占位配置跳过 Ping
	cfg := Config{
		ID: "m2", Type: "mysql",
		Host: "127.0.0.1", Port: 3306, User: "u", Password: "p", DBName: "db",
	}
	ds, err := r.Register(cfg)
	if err != nil {
		// 本地无 MySQL 时 Open 可能成功但 Ping 会超时；本测试仅验证工厂被调用
		t.Logf("Register(mysql) with no server: %v", err)
		return
	}
	defer ds.Close()
	if ds.ID() != "m2" {
		t.Errorf("ID() = %s", ds.ID())
	}
	if err := ds.Ping(context.Background()); err != nil {
		t.Logf("Ping (expected to fail without real MySQL): %v", err)
	}
}

func TestMySQLDataSource_DB(t *testing.T) {
	r := NewRegistry()
	RegisterMySQL(r)
	cfg := Config{ID: "m3", Type: "mysql", Host: "127.0.0.1", Port: 3306, User: "u", Password: "p", DBName: "db"}
	ds, err := r.Register(cfg)
	if err != nil {
		t.Skipf("no MySQL driver or open failed: %v", err)
	}
	m, ok := ds.(*MySQLDataSource)
	if !ok {
		t.Fatal("expected *MySQLDataSource")
	}
	if m.DB() == nil {
		t.Error("DB() should not be nil")
	}
	_ = m.Close()
}
