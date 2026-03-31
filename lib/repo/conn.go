package repo

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/XSAM/otelsql"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Conn struct {
	conn *sql.DB
}

func (c *Conn) DB() *sql.DB {
	return c.conn
}

func (c *Conn) Close() {
	_ = c.conn.Close()
}

func NewDatabase(connStr string) (*Conn, error) {
	driverName, err := otelsql.Register("postgres", otelsql.WithAttributes(semconv.DBSystemPostgreSQL))
	if err != nil {
		driverName = "postgres"
	}
	db, err := sql.Open(driverName, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	return &Conn{conn: db}, nil
}
