package createdb

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func CreateTableWorkouts(ctx context.Context, conn *pgx.Conn) error {
	//агругумент у VARCHAR - максимальное значение символов
	sqlQuery := `
	CREATE TABLE IF NOT EXISTS workouts(
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		distance DECIMAL(7, 2),
		duration TIMESTAMP NOT NULL,
		created_at TIMESTAMP NOT NULL,

		CONSTRAINT fk_workouts_users
                FOREIGN KEY (user_id) 
                REFERENCES users(id)
                ON DELETE RESTRICT
	);
	`

	_, err := conn.Exec(ctx, sqlQuery)

	if err != nil {
		return err
	}

	return nil
}
