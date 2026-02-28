package metadata

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sath/datasource"
)

func TestFetchSchema_mock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	mock.ExpectQuery("SELECT DATABASE()").WillReturnRows(sqlmock.NewRows([]string{"db"}).AddRow("mydb"))
	mock.ExpectQuery("SELECT TABLE_NAME FROM information_schema.TABLES").WithArgs("mydb").WillReturnRows(
		sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("users").AddRow("orders"))
	mock.ExpectQuery("SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY").WithArgs("mydb", "users").WillReturnRows(
		sqlmock.NewRows([]string{"COLUMN_NAME", "COLUMN_TYPE", "IS_NULLABLE", "COLUMN_KEY"}).
			AddRow("id", "int", "NO", "PRI").
			AddRow("name", "varchar(64)", "YES", ""))
	mock.ExpectQuery("SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY").WithArgs("mydb", "orders").WillReturnRows(
		sqlmock.NewRows([]string{"COLUMN_NAME", "COLUMN_TYPE", "IS_NULLABLE", "COLUMN_KEY"}).
			AddRow("id", "int", "NO", "PRI"))

	sch, err := FetchSchema(ctx, db)
	if err != nil {
		t.Fatalf("FetchSchema: %v", err)
	}
	if sch.Name != "mydb" {
		t.Errorf("Name = %s", sch.Name)
	}
	if len(sch.Tables) != 2 {
		t.Fatalf("len(Tables) = %d", len(sch.Tables))
	}
	if sch.Tables[0].Name != "users" || len(sch.Tables[0].Columns) != 2 {
		t.Errorf("users table: %+v", sch.Tables[0])
	}
	if sch.Tables[0].Columns[0].Name != "id" || !sch.Tables[0].Columns[0].PrimaryKey {
		t.Errorf("users.id: %+v", sch.Tables[0].Columns[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestRefreshFromRegistry_unsupported(t *testing.T) {
	r := datasource.NewRegistry()
	datasource.RegisterNoop(r)
	_, _ = r.Register(datasource.Config{ID: "n1", Type: "noop"})
	store := NewInMemoryStore()
	ctx := context.Background()
	err := RefreshFromRegistry(ctx, r, store, "n1")
	if err != ErrUnsupportedDataSource {
		t.Errorf("RefreshFromRegistry(noop) = %v, want ErrUnsupportedDataSource", err)
	}
}

func TestRefreshFromRegistry_notFound(t *testing.T) {
	r := datasource.NewRegistry()
	store := NewInMemoryStore()
	ctx := context.Background()
	err := RefreshFromRegistry(ctx, r, store, "missing")
	if err != datasource.ErrNotFound {
		t.Errorf("RefreshFromRegistry(missing) = %v, want ErrNotFound", err)
	}
}

// TestRefreshFromRegistry_mysql 需要 *datasource.MySQLDataSource，用真实 DB 或跳过
func TestRefreshFromRegistry_mysql_integration(t *testing.T) {
	r := datasource.NewRegistry()
	datasource.RegisterMySQL(r)
	cfg := datasource.Config{ID: "mi", Type: "mysql", Host: "127.0.0.1", Port: 3306, User: "u", Password: "p", DBName: "test"}
	ds, err := r.Register(cfg)
	if err != nil {
		t.Skipf("no MySQL: %v", err)
	}
	defer ds.Close()
	store := NewInMemoryStore()
	ctx := context.Background()
	err = RefreshFromRegistry(ctx, r, store, "mi")
	if err != nil {
		t.Logf("RefreshFromRegistry (no real MySQL): %v", err)
		return
	}
	sch, _ := store.GetSchema(ctx, "mi")
	if sch != nil {
		t.Logf("schema: %s, tables: %d", sch.Name, len(sch.Tables))
	}
}
