package ledger

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var DB *sql.DB

func InitDB(ctx context.Context) (*sql.DB, error) {
	if DB != nil {
		return DB, nil
	}

	db, err := sql.Open("pgx", databaseDSN())
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	DB = db
	return DB, nil
}

func SetDB(db *sql.DB) {
	DB = db
}

func CloseDB() error {
	if DB == nil {
		return nil
	}
	err := DB.Close()
	DB = nil
	return err
}

func databaseDSN() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}

	host := envOrDefault("DB_HOST", "localhost")
	port := envOrDefault("DB_PORT", "5432")
	user := envOrDefault("DB_USER", "postgres")
	password := envOrDefault("DB_PASS", "postgres")
	database := envOrDefault("DB_NAME", "cashapp")

	values := url.Values{}
	values.Set("sslmode", "disable")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?%s",
		url.PathEscape(user), url.PathEscape(password), host, port,
		url.PathEscape(database), values.Encode())
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func redisDBNumber() int {
	value, err := strconv.Atoi(envOrDefault("REDIS_DB", "0"))
	if err != nil || value < 0 {
		return 0
	}
	return value
}
