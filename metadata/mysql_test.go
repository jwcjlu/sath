package metadata

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sath/datasource"
)

func TestFetchSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT DATABASE").
		WillReturnRows(sqlmock.NewRows([]string{"DATABASE()"}).AddRow("testdb"))

	rows := sqlmock.NewRows([]string{
		"table_name", "table_comment", "column_name", "data_type", "is_nullable", "column_comment",
	}).
		AddRow("users", "user table", "id", "int", "NO", "primary key").
		AddRow("users", "user table", "name", "varchar", "YES", "user name")

	mock.ExpectQuery("FROM information_schema.tables").
		WithArgs("testdb").
		WillReturnRows(rows)

	schema, err := FetchSchema(context.Background(), db)
	if err != nil {
		t.Fatalf("FetchSchema error: %v", err)
	}
	if schema == nil || schema.Name != "testdb" {
		t.Fatalf("unexpected schema: %+v", schema)
	}
	if len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(schema.Tables))
	}
	tbl := schema.Tables[0]
	if tbl.Name != "users" || len(tbl.Columns) != 2 {
		t.Fatalf("unexpected table: %+v", tbl)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

type dsWithDB struct {
	id string
	db *sql.DB
}

func (d *dsWithDB) ID() string                     { return d.id }
func (d *dsWithDB) Ping(ctx context.Context) error { return nil }
func (d *dsWithDB) Close() error                   { return nil }
func (d *dsWithDB) DB() *sql.DB                    { return d.db }

func TestRefreshFromRegistry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT DATABASE").
		WillReturnRows(sqlmock.NewRows([]string{"DATABASE()"}).AddRow("testdb"))

	rows := sqlmock.NewRows([]string{
		"table_name", "table_comment", "column_name", "data_type", "is_nullable", "column_comment",
	}).
		AddRow("users", "", "id", "int", "NO", "")

	mock.ExpectQuery("FROM information_schema.tables").
		WithArgs("testdb").
		WillReturnRows(rows)

	reg := datasource.NewRegistry()
	reg.RegisterType("test", func(cfg datasource.Config) (datasource.DataSource, error) {
		return &dsWithDB{id: cfg.ID, db: db}, nil
	})
	if _, err := reg.Register(datasource.Config{ID: "test", Type: "test"}); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	store := NewInMemoryStore(nil)
	schema, err := RefreshFromRegistry(context.Background(), reg, store, "test")
	if err != nil {
		t.Fatalf("RefreshFromRegistry error: %v", err)
	}
	if schema == nil || schema.Name != "testdb" {
		t.Fatalf("unexpected schema: %+v", schema)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

type dsNoDB struct{ id string }

func (d *dsNoDB) ID() string                     { return d.id }
func (d *dsNoDB) Ping(ctx context.Context) error { return nil }
func (d *dsNoDB) Close() error                   { return nil }

func TestRefreshFromRegistry_Unsupported(t *testing.T) {
	reg := datasource.NewRegistry()
	reg.RegisterType("no-db", func(cfg datasource.Config) (datasource.DataSource, error) {
		return &dsNoDB{id: cfg.ID}, nil
	})
	if _, err := reg.Register(datasource.Config{ID: "no-db", Type: "no-db"}); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	store := NewInMemoryStore(nil)
	_, err := RefreshFromRegistry(context.Background(), reg, store, "no-db")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err != ErrUnsupportedDataSource {
		t.Fatalf("expected ErrUnsupportedDataSource, got %v", err)
	}
}
