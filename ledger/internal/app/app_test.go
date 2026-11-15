package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"ledger/internal/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func mockDatabase(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		_ = db.Close()
	})
	return db, mock
}

func databaseOpener(t *testing.T, db *sql.DB, wantDSN string) func(string, string) (*sql.DB, error) {
	t.Helper()
	return func(driver, dsn string) (*sql.DB, error) {
		if driver != "pgx" || dsn != wantDSN {
			t.Fatalf("Open arguments: driver = %q, DSN matches = %v", driver, dsn == wantDSN)
		}
		return db, nil
	}
}

func TestFactoryBuildsWorkingService(t *testing.T) {
	server := miniredis.RunT(t)
	server.RequireAuth("secret")
	db, mock := mockDatabase(t)
	mock.ExpectPing()
	cfg := config{databaseDSN: "test-dsn", redisAddr: server.Addr(), redisPassword: "secret", redisDB: 2}
	svc, closeFn, err := newWithConfig(t.Context(), cfg, databaseOpener(t, db, cfg.databaseDSN))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeFn() })
	if svc == nil || closeFn == nil {
		t.Fatal("factory must return service and cleanup function")
	}

	mock.ExpectExec("INSERT INTO budgets").WithArgs("еда", 500.0).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := svc.SetBudget(t.Context(), domain.Budget{Category: "еда", Limit: 500}); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT limit_amount").WithArgs("еда").WillReturnRows(sqlmock.NewRows([]string{"limit_amount"}).AddRow(500))
	mock.ExpectQuery("SELECT COALESCE").WithArgs("еда").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(400))
	mock.ExpectQuery("INSERT INTO expenses").WithArgs(100.0, "еда", "", "2025-09-10").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	tx, err := svc.AddTransaction(t.Context(), domain.Transaction{Amount: 100, Category: "еда", Date: "2025-09-10"})
	if err != nil || tx.ID != 42 {
		t.Fatalf("AddTransaction() = %+v, %v", tx, err)
	}
	mock.ExpectQuery("SELECT id, amount").WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "category", "description", "date"}).AddRow(42, 100, "еда", "", "2025-09-10"))
	if transactions, err := svc.ListTransactions(t.Context()); err != nil || len(transactions) != 1 || transactions[0] != tx {
		t.Fatalf("ListTransactions() = %+v, %v", transactions, err)
	}
	mock.ExpectQuery("SELECT category, limit_amount").WillReturnRows(sqlmock.NewRows([]string{"category", "limit_amount"}).AddRow("еда", 500))
	for range 2 {
		budgets, err := svc.ListBudgets(t.Context())
		if err != nil || len(budgets) != 1 || budgets[0].Limit != 500 {
			t.Fatalf("ListBudgets() = %+v, %v", budgets, err)
		}
	}
	if !server.DB(2).Exists("budgets:all") || server.DB(0).Exists("budgets:all") {
		t.Fatal("budget cache must use the configured Redis database")
	}
	mock.ExpectQuery("SELECT DISTINCT category").WithArgs("2025-09-01", "2025-09-30").WillReturnRows(sqlmock.NewRows([]string{"category"}).AddRow("еда"))
	mock.ExpectQuery("SELECT COALESCE").WithArgs("еда", "2025-09-01", "2025-09-30").WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(500))
	for range 2 {
		report, err := svc.GetReportSummary(t.Context(), "2025-09-01", "2025-09-30")
		if err != nil || len(report) != 1 || report[0].Amount != 500 {
			t.Fatalf("GetReportSummary() = %+v, %v", report, err)
		}
	}
	mock.ExpectClose()
	for range 2 {
		if err := closeFn(); err != nil {
			t.Fatalf("closeFn() = %v", err)
		}
	}
	if _, err := svc.ListBudgets(t.Context()); !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("Redis must be closed; error = %v", err)
	}
	if err := db.PingContext(t.Context()); err == nil {
		t.Fatal("PostgreSQL must be closed")
	}
}

func TestFactoryOpenError(t *testing.T) {
	want := errors.New("open failed")
	svc, closeFn, err := newWithConfig(t.Context(), config{}, func(string, string) (*sql.DB, error) { return nil, want })
	if svc != nil || closeFn != nil || !errors.Is(err, want) {
		t.Fatalf("factory failure: service = %v, closeFn present = %v, error = %v", svc, closeFn != nil, err)
	}
}

func TestFactoryPostgresPingErrorClosesDatabase(t *testing.T) {
	db, mock := mockDatabase(t)
	want := errors.New("database unavailable")
	mock.ExpectPing().WillReturnError(want)
	mock.ExpectClose()
	svc, closeFn, err := newWithConfig(t.Context(), config{}, databaseOpener(t, db, ""))
	if svc != nil || closeFn != nil || !errors.Is(err, want) {
		t.Fatalf("factory failure: service = %v, closeFn present = %v, error = %v", svc, closeFn != nil, err)
	}
}

func TestFactoryPropagatesCanceledContext(t *testing.T) {
	db, mock := mockDatabase(t)
	mock.ExpectClose()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	svc, closeFn, err := newWithConfig(ctx, config{}, databaseOpener(t, db, ""))
	if svc != nil || closeFn != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("factory must fail with canceled context: %v", err)
	}
}

func TestFactoryRedisPingErrorClosesResources(t *testing.T) {
	server := miniredis.RunT(t)
	server.RequireAuth("required-password")
	db, mock := mockDatabase(t)
	mock.ExpectPing()
	mock.ExpectClose()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	svc, closeFn, err := newWithConfig(ctx, config{redisAddr: server.Addr()}, databaseOpener(t, db, ""))
	if svc != nil || closeFn != nil || err == nil || !strings.HasPrefix(err.Error(), "ping Redis:") {
		t.Fatalf("factory failure: service = %v, closeFn present = %v, error = %v", svc, closeFn != nil, err)
	}
	// The Redis client closes its socket synchronously, but the server observes
	// the disconnect asynchronously.
	deadline := time.Now().Add(time.Second)
	for server.CurrentConnectionCount() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("Redis connections were not closed after failed startup")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestFactoryCloseError(t *testing.T) {
	server := miniredis.RunT(t)
	db, mock := mockDatabase(t)
	mock.ExpectPing()
	want := errors.New("close failed")
	mock.ExpectClose().WillReturnError(want)
	_, closeFn, err := newWithConfig(t.Context(), config{redisAddr: server.Addr()}, databaseOpener(t, db, ""))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := closeFn(); !errors.Is(err, want) {
			t.Fatalf("closeFn error = %v, want wrapped close error", err)
		}
	}
}
