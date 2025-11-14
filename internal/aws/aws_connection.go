package aws

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type DBI struct {
	User     string
	Password string
	Endpoint string
	Port     int
	Database string
}

func CreateConnection(i DBI) (*sqlx.DB, error) {
	// DSN (Data Source Name)
	// (수정 1: parseTime=true 및 charset=utf8mb4 추가)
	DSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		i.User, i.Password, i.Endpoint, i.Port, i.Database)

	// sqlx.Connect
	db, err := sqlx.Connect("mysql", DSN)
	if err != nil {
		return nil, err
	}

	return db, nil
}