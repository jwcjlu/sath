package executor

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sath/datasource"
)

type dsWithDB struct {
	id string
	db *sql.DB
}

func (d *dsWithDB) ID() string                     { return d.id }
func (d *dsWithDB) Ping(ctx context.Context) error { return nil }
func (d *dsWithDB) Close() error                   { return nil }
func (d *dsWithDB) DB() *sql.DB                    { return d.db }

func TestIsWriteDSL(t *testing.T) {
	tests := []struct {
		dsl   string
		write bool
	}{
		{"SELECT 1", false},
		{"  select * from t", false},
		{"INSERT INTO t VALUES (1)", true},
		{"  update t set a=1", true},
		{"delete from t", true},
		{"REPLACE INTO t VALUES (1)", true},
		{"CREATE TABLE t (id int)", true},
		{"DROP TABLE t", true},
		{"ALTER TABLE t ADD c int", true},
		{"TRUNCATE t", true},
		{"", false},
	}
	for _, tt := range tests {
		got := isWriteDSL(tt.dsl)
		if got != tt.write {
			t.Errorf("isWriteDSL(%q) = %v, want %v", tt.dsl, got, tt.write)
		}
	}
}

func TestMySQLExecutor_Execute_ReadOnly_RejectsWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	reg := datasource.NewRegistry()
	reg.RegisterType("mysql", func(cfg datasource.Config) (datasource.DataSource, error) {
		return &dsWithDB{id: cfg.ID, db: db}, nil
	})
	if _, err := reg.Register(datasource.Config{ID: "ds1", Type: "mysql"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ex := NewMySQLExecutor(reg)
	ctx := context.Background()
	opts := ExecuteOptions{ReadOnly: true}

	_, err = ex.Execute(ctx, "ds1", "INSERT INTO t (a) VALUES (1)", opts)
	if err == nil {
		t.Fatal("expected error for INSERT in read-only mode")
	}
	if !errors.Is(err, ErrReadOnlyViolation) {
		t.Errorf("expected ErrReadOnlyViolation, got %v", err)
	}

	_, err = ex.Execute(ctx, "ds1", "UPDATE t SET a=1", opts)
	if !errors.Is(err, ErrReadOnlyViolation) {
		t.Errorf("expected ErrReadOnlyViolation for UPDATE, got %v", err)
	}

	// SELECT 不应触发写拒绝；mock 不期望任何 DB 调用若我们只测到上面就返回
	// 下面单独测 SELECT 会执行
	mock.ExpectQuery("SELECT 1").
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	res, err := ex.Execute(ctx, "ds1", "SELECT 1", opts)
	if err != nil {
		t.Fatalf("SELECT in read-only: %v", err)
	}
	if len(res.Rows) != 1 || len(res.Columns) != 1 {
		t.Errorf("unexpected result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestMySQLExecutor_Execute_Query_MaxRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "a").
		AddRow(2, "b").
		AddRow(3, "c")
	mock.ExpectQuery("SELECT id, name FROM users").
		WillReturnRows(rows)

	reg := datasource.NewRegistry()
	reg.RegisterType("mysql", func(cfg datasource.Config) (datasource.DataSource, error) {
		return &dsWithDB{id: cfg.ID, db: db}, nil
	})
	if _, err := reg.Register(datasource.Config{ID: "ds1", Type: "mysql"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ex := NewMySQLExecutor(reg)
	res, err := ex.Execute(context.Background(), "ds1", "SELECT id, name FROM users", ExecuteOptions{MaxRows: 2})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Columns) != 2 || res.Columns[0] != "id" || res.Columns[1] != "name" {
		t.Errorf("Columns: %v", res.Columns)
	}
	if len(res.Rows) != 2 {
		t.Errorf("expected 2 rows (MaxRows=2), got %d", len(res.Rows))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestMySQLExecutor_Execute_Write_AffectedRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE users SET name").
		WillReturnResult(sqlmock.NewResult(0, 3))

	reg := datasource.NewRegistry()
	reg.RegisterType("mysql", func(cfg datasource.Config) (datasource.DataSource, error) {
		return &dsWithDB{id: cfg.ID, db: db}, nil
	})
	if _, err := reg.Register(datasource.Config{ID: "ds1", Type: "mysql"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ex := NewMySQLExecutor(reg)
	res, err := ex.Execute(context.Background(), "ds1", "UPDATE users SET name = 'x'", ExecuteOptions{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.AffectedRows != 3 {
		t.Errorf("AffectedRows = %d, want 3", res.AffectedRows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestMySQLExecutor_Execute_UnsupportedDataSource(t *testing.T) {
	reg := datasource.NewRegistry()
	datasource.RegisterNoop(reg)
	if _, err := reg.Register(datasource.Config{ID: "noop", Type: "noop"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ex := NewMySQLExecutor(reg)
	_, err := ex.Execute(context.Background(), "noop", "SELECT 1", ExecuteOptions{})
	if err == nil {
		t.Fatal("expected error for datasource without DB()")
	}
	if !errors.Is(err, ErrUnsupportedDataSource) {
		t.Errorf("expected ErrUnsupportedDataSource, got %v", err)
	}
}

func TestMySQLExecutor_Execute_Timeout(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT SLEEP").
		WillDelayFor(100 * 1e9) // 100s delay; context will cancel first
	// 若驱动支持 context 取消，会提前返回；这里仅验证带 Timeout 的 opts 能传下去且不 panic
	reg := datasource.NewRegistry()
	reg.RegisterType("mysql", func(cfg datasource.Config) (datasource.DataSource, error) {
		return &dsWithDB{id: cfg.ID, db: db}, nil
	})
	if _, err := reg.Register(datasource.Config{ID: "ds1", Type: "mysql"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	ex := NewMySQLExecutor(reg)
	ctx, cancel := context.WithTimeout(context.Background(), 0) // 立即过期
	defer cancel()
	_, _ = ex.Execute(ctx, "ds1", "SELECT SLEEP(100)", ExecuteOptions{Timeout: 10})
	// 期望因 context 已取消而得到错误（不断言具体错误，仅保证不 panic）
}
