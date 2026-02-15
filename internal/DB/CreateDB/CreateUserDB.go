package createdb

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func CreateTableUsers(ctx context.Context, conn *pgx.Conn) error {
	//агругумент у VARCHAR - максимальное значение символов
	sqlQuery := `
	CREATE TABLE IF NOT EXISTS users(
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(150) NOT NULL,
		password VARCHAR(150) NOT NULL,
		created_at TIMESTAMP NOT NULL,
		
		UNIQUE(email)
	);
	`

	_, err := conn.Exec(ctx, sqlQuery)

	if err != nil {
		return err
	}

	return nil
}
