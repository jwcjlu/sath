package executor

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sath/datasource"
)

func TestMySQLExecutor_Execute_readOnlyRejectsWrite(t *testing.T) {
	r := datasource.NewRegistry()
	datasource.RegisterNoop(r)
	_, _ = r.Register(datasource.Config{ID: "n1", Type: "noop"})
	ex := &MySQLExecutor{Registry: r}
	ctx := context.Background()
	res, err := ex.Execute(ctx, "n1", "INSERT INTO t (a) VALUES (1)", ExecuteOptions{ReadOnly: true})
	if err == nil {
		t.Fatal("expected error for write in read-only mode")
	}
	if res == nil || res.Error == "" {
		t.Error("expected Result with Error set")
	}
}

func TestMySQLExecutor_Execute_notMySQL(t *testing.T) {
	r := datasource.NewRegistry()
	datasource.RegisterNoop(r)
	_, _ = r.Register(datasource.Config{ID: "n1", Type: "noop"})
	ex := &MySQLExecutor{Registry: r}
	ctx := context.Background()
	_, err := ex.Execute(ctx, "n1", "SELECT 1", ExecuteOptions{})
	if err == nil {
		t.Fatal("expected error for non-MySQL datasource")
	}
}

func TestMySQLExecutor_Execute_query_maxRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	r := datasource.NewRegistry()
	ds := datasource.NewMySQLDataSourceFromDB("m1", db)
	r.RegisterType("mysql", func(cfg datasource.Config) (datasource.DataSource, error) {
		return ds, nil
	})
	_, _ = r.Register(datasource.Config{ID: "m1", Type: "mysql"})
	ex := &MySQLExecutor{Registry: r}
	ctx := context.Background()

	mock.ExpectQuery("SELECT 1, 2").WillReturnRows(
		sqlmock.NewRows([]string{"a", "b"}).AddRow(1, 2).AddRow(3, 4).AddRow(5, 6))
	res, err := ex.Execute(ctx, "m1", "SELECT 1, 2", ExecuteOptions{MaxRows: 2})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Rows) != 2 {
		t.Errorf("MaxRows=2: got %d rows", len(res.Rows))
	}
	if res.Columns[0] != "a" || res.Columns[1] != "b" {
		t.Errorf("Columns = %v", res.Columns)
	}
	_ = mock.ExpectationsWereMet()
}

func TestMySQLExecutor_Execute_write(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	r := datasource.NewRegistry()
	ds := datasource.NewMySQLDataSourceFromDB("m1", db)
	r.RegisterType("mysql", func(cfg datasource.Config) (datasource.DataSource, error) {
		return ds, nil
	})
	_, _ = r.Register(datasource.Config{ID: "m1", Type: "mysql"})
	ex := &MySQLExecutor{Registry: r}
	ctx := context.Background()

	mock.ExpectExec("UPDATE t SET x = 1").WillReturnResult(sqlmock.NewResult(0, 3))
	res, err := ex.Execute(ctx, "m1", "UPDATE t SET x = 1", ExecuteOptions{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.AffectedRows != 3 {
		t.Errorf("AffectedRows = %d", res.AffectedRows)
	}
	_ = mock.ExpectationsWereMet()
}