package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// PostgresTestDB хранит инстанс контейнера и пул соединений для тестов
type PostgresTestDB struct {
	Container  *tcpostgres.PostgresContainer
	Pool       *pgxpool.Pool
	ConnString string
}

// SetupTestPostgres запускает контейнер с PostgreSQL 16+, применяет миграции и возвращает PostgresTestDB
func SetupTestPostgres(ctx context.Context) (*PostgresTestDB, error) {
	if os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}

	dbName := "smartrun_test"
	dbUser := "postgres"
	dbPassword := "postgres"

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase(dbName),
		tcpostgres.WithUsername(dbUser),
		tcpostgres.WithPassword(dbPassword),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = testcontainers.TerminateContainer(pgContainer)
		return nil, fmt.Errorf("get container connection string: %w", err)
	}

	// Применяем миграции
	if err := runMigrations(connStr); err != nil {
		_ = testcontainers.TerminateContainer(pgContainer)
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	// Инициализируем пул соединений pgxpool
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		_ = testcontainers.TerminateContainer(pgContainer)
		return nil, fmt.Errorf("parse pool config: %w", err)
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		_ = testcontainers.TerminateContainer(pgContainer)
		return nil, fmt.Errorf("create pgxpool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_ = testcontainers.TerminateContainer(pgContainer)
		return nil, fmt.Errorf("ping pgxpool: %w", err)
	}

	return &PostgresTestDB{
		Container:  pgContainer,
		Pool:       pool,
		ConnString: connStr,
	}, nil
}

// Close завершает соединения пула и останавливает контейнер
func (db *PostgresTestDB) Close(ctx context.Context) error {
	if db.Pool != nil {
		db.Pool.Close()
	}
	if db.Container != nil {
		return testcontainers.TerminateContainer(db.Container)
	}
	return nil
}

// TruncateTables очищает таблицы базы данных между тестами
func (db *PostgresTestDB) TruncateTables(ctx context.Context) error {
	queries := []string{
		"TRUNCATE TABLE daily_metrics, metrics, password_resets, sessions, user_goals, workouts, users RESTART IDENTITY CASCADE;",
	}

	for _, q := range queries {
		if _, err := db.Pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("truncate tables: %w", err)
		}
	}
	return nil
}

// CreateUser создает тестового пользователя и возвращает его ID
func (db *PostgresTestDB) CreateUser(ctx context.Context, name, email string) (int64, error) {
	var id int64
	query := `
		INSERT INTO users (name, email, password, totp_secret, totp_enables)
		VALUES ($1, $2, $3, '', false)
		RETURNING id;
	`
	err := db.Pool.QueryRow(ctx, query, name, email, "hashed_password").Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create test user: %w", err)
	}
	return id, nil
}

// runMigrations применяет все миграции из каталога migrations/ с помощью golang-migrate
func runMigrations(connStr string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}

	migrationsDir := filepath.Join(projectRoot, "migrations")
	d, err := iofs.New(os.DirFS(migrationsDir), ".")
	if err != nil {
		return fmt.Errorf("create iofs source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, connStr)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}

	return nil
}

// findProjectRoot ищет корень проекта по наличию go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find project root (go.mod not found)")
		}
		dir = parent
	}
}
