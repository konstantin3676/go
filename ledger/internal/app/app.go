package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"ledger/internal/repository/cached"
	"ledger/internal/repository/pg"
	"ledger/internal/service"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

func New(ctx context.Context) (service.Service, func() error, error) {
	return newWithConfig(ctx, configFromEnv(), sql.Open)
}

func newWithConfig(ctx context.Context, cfg config, openDB func(string, string) (*sql.DB, error)) (service.Service, func() error, error) {
	db, err := openDB("pgx", cfg.databaseDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("open PostgreSQL connection: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.redisAddr,
		Password: cfg.redisPassword,
		DB:       cfg.redisDB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping Redis: %w", err)
	}

	svc := service.New(
		cached.NewBudgetRepository(pg.NewBudgetRepository(db), client),
		pg.NewTransactionRepository(db),
		cached.NewReportRepository(pg.NewReportRepository(db), client),
	)
	var once sync.Once
	var closeErr error
	closeFn := func() error {
		once.Do(func() {
			closeErr = errors.Join(client.Close(), db.Close())
		})
		return closeErr
	}
	return svc, closeFn, nil
}
