package db

import (
	"context"
	"errors"
	"os"

	"github.com/jackc/pgx/v5"
)

//postgres://username:password@localhost:5432/database_name?sslmode=disable

func CreateConnection(ctx context.Context) (*pgx.Conn, error) {
	connString := os.Getenv("CONN_STRING")
	if connString == "" {
		return nil, errors.New("CONN_STRING environment variable is not set")
	}
	return pgx.Connect(ctx, connString)

}
