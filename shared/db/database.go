package db

import (
	"database/sql"
)

type Database interface {
	Connect() error
	Close() error
	DB() *sql.DB
}
