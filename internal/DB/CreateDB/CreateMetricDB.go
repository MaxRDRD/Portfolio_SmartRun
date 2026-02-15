package createdb

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func CreateTableMetrics(ctx context.Context, conn *pgx.Conn) error {
	sqlQuery := `
	CREATE TABLE IF NOT EXISTS metrics(
		id SERIAL PRIMARY KEY,
		workouts_id INTEGER NOT NULL,
		pace INTEGER NOT NULL,
		time_run TIMESTAMP NOT NULL,
		running_pace INTEGER GENERATED ALWAYS AS (
            time_run / pace
        ) STORED,
		created_at TIMESTAMP NOT NULL
	)
	`

	_, err := conn.Exec(ctx, sqlQuery)

	if err != nil {
		return err
	}

	return nil
}
