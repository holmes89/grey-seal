package repo

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Conn struct {
	conn *sql.DB
	path string
}

func (conn *Conn) DB() *sql.DB {
	return conn.conn
}

func NewDatabase(path string, migrate ...bool) (*Conn, error) {
	db, err := retryConn(3, 10*time.Second, func() (db *sql.DB, e error) {
		log.Print("connecting...")
		conn, err := sql.Open("postgres", fmt.Sprintf("postgres://%s", path))
		if err != nil {
			return nil, err
		}
		return conn, conn.Ping()
	})
	log.Print("connected to postgres")
	if err != nil {
		log.Fatal(err)
	}
	st := &Conn{
		conn: db,
		path: path,
	}
	if len(migrate) == 0 || migrate[0] {
		migrateDB(db)
	}
	return st, nil
}

func (conn *Conn) Close() error {
	return conn.conn.Close()
}

// TODO migrations
// rename functions to match interfaces
func migrateDB(conn *sql.DB) {

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	if err := goose.Up(conn, "migrations"); err != nil {
		panic(err)
	}
}

func retryConn(attempts int, sleep time.Duration, callback func() (*sql.DB, error)) (*sql.DB, error) {
	for i := 0; i <= attempts; i++ {
		conn, err := callback()
		if err == nil {
			return conn, nil
		}
		log.Print(err)
		time.Sleep(sleep)

		log.Print("error connecting, retrying")
	}
	return nil, fmt.Errorf("after %d attempts, connection failed", attempts)
}
