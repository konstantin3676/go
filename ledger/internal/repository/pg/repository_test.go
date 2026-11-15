package pg

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"ledger/internal/domain"

	"github.com/DATA-DOG/go-sqlmock"
)

func mockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
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

func TestBudgetUpsert(t *testing.T) {
	db, mock := mockDB(t)
	mock.ExpectExec(`INSERT INTO budgets\(category, limit_amount\).*ON CONFLICT \(category\) DO UPDATE SET limit_amount = EXCLUDED.limit_amount`).
		WithArgs("еда", 500.0).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := NewBudgetRepository(db).Upsert(t.Context(), domain.Budget{Category: "еда", Limit: 500}); err != nil {
		t.Fatal(err)
	}
}

func TestBudgetGetByCategory(t *testing.T) {
	for _, found := range []bool{true, false} {
		name := "found"
		if !found {
			name = "not found"
		}
		t.Run(name, func(t *testing.T) {
			db, mock := mockDB(t)
			rows := sqlmock.NewRows([]string{"limit_amount"})
			if found {
				rows.AddRow(500.0)
			}
			mock.ExpectQuery(`SELECT limit_amount FROM budgets WHERE category = \$1`).WithArgs("еда").WillReturnRows(rows)
			got, err := NewBudgetRepository(db).GetByCategory(t.Context(), "еда")
			if !found {
				if !errors.Is(err, domain.ErrBudgetNotFound) {
					t.Fatalf("error = %v, want ErrBudgetNotFound", err)
				}
				return
			}
			if err != nil || got != (domain.Budget{Category: "еда", Limit: 500}) {
				t.Fatalf("GetByCategory() = %+v, %v", got, err)
			}
		})
	}
}

func TestBudgetList(t *testing.T) {
	db, mock := mockDB(t)
	mock.ExpectQuery(`SELECT category, limit_amount FROM budgets ORDER BY category`).
		WillReturnRows(sqlmock.NewRows([]string{"category", "limit_amount"}).AddRow("еда", 500).AddRow("транспорт", 200))
	want := []domain.Budget{{Category: "еда", Limit: 500}, {Category: "транспорт", Limit: 200}}
	got, err := NewBudgetRepository(db).List(t.Context())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %+v, %v; want %+v", got, err, want)
	}
}

func TestTransactionAdd(t *testing.T) {
	db, mock := mockDB(t)
	tx := domain.Transaction{Amount: 100, Category: "еда", Description: "ланч", Date: "2025-09-10"}
	mock.ExpectQuery(`INSERT INTO expenses\(amount, category, description, date\).*RETURNING id`).
		WithArgs(tx.Amount, tx.Category, tx.Description, tx.Date).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	got, err := NewTransactionRepository(db).Add(t.Context(), tx)
	tx.ID = 42
	if err != nil || got != tx {
		t.Fatalf("Add() = %+v, %v; want %+v", got, err, tx)
	}
}

func TestTransactionList(t *testing.T) {
	db, mock := mockDB(t)
	mock.ExpectQuery(`SELECT id, amount, category, COALESCE\(description, ''\), date::text FROM expenses ORDER BY date DESC, id DESC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "category", "description", "date"}).
			AddRow(2, 150, "еда", "", "2025-09-10").AddRow(1, 100, "еда", "ланч", "2025-09-10"))
	want := []domain.Transaction{
		{ID: 2, Amount: 150, Category: "еда", Date: "2025-09-10"},
		{ID: 1, Amount: 100, Category: "еда", Description: "ланч", Date: "2025-09-10"},
	}
	got, err := NewTransactionRepository(db).List(t.Context())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %+v, %v; want %+v", got, err, want)
	}
}

func TestTransactionSumByCategory(t *testing.T) {
	cases := []struct {
		name string
		sum  float64
	}{
		{name: "empty category", sum: 0},
		{name: "existing expenses", sum: 250.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := mockDB(t)
			mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount\), 0\) FROM expenses WHERE category = \$1`).
				WithArgs("еда").WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(tc.sum))
			got, err := NewTransactionRepository(db).SumByCategory(t.Context(), "еда")
			if err != nil || got != tc.sum {
				t.Fatalf("SumByCategory() = %v, %v; want %v", got, err, tc.sum)
			}
		})
	}
}

func TestReportCategories(t *testing.T) {
	db, mock := mockDB(t)
	mock.ExpectQuery(`SELECT DISTINCT category FROM expenses WHERE date BETWEEN \$1 AND \$2 ORDER BY category`).
		WithArgs("2025-09-01", "2025-09-30").
		WillReturnRows(sqlmock.NewRows([]string{"category"}).AddRow("еда"))
	got, err := NewReportRepository(db).Categories(t.Context(), "2025-09-01", "2025-09-30")
	want := []string{"еда"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Categories() = %+v, %v; want %+v", got, err, want)
	}
}

func TestReportSumByCategory(t *testing.T) {
	db, mock := mockDB(t)
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount\), 0\) FROM expenses WHERE category = \$1 AND date BETWEEN \$2 AND \$3`).
		WithArgs("еда", "2025-09-01", "2025-09-30").
		WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(250.5))
	got, err := NewReportRepository(db).SumByCategory(t.Context(), "еда", "2025-09-01", "2025-09-30")
	if err != nil || got != 250.5 {
		t.Fatalf("SumByCategory() = %v, %v", got, err)
	}
}

func TestReportQueriesStopOnDeadline(t *testing.T) {
	for _, categories := range []bool{true, false} {
		t.Run(map[bool]string{true: "categories", false: "sum"}[categories], func(t *testing.T) {
			db, mock := mockDB(t)
			mock.ExpectQuery("SELECT").WillDelayFor(time.Second).
				WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(0))
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()
			start := time.Now()
			var err error
			if categories {
				_, err = NewReportRepository(db).Categories(ctx, "2025-09-01", "2025-09-30")
			} else {
				_, err = NewReportRepository(db).SumByCategory(ctx, "еда", "2025-09-01", "2025-09-30")
			}
			if err == nil || ctx.Err() != context.DeadlineExceeded || time.Since(start) >= time.Second {
				t.Fatalf("query did not stop on deadline: %v", err)
			}
		})
	}
}

func TestReportCategoryRowErrors(t *testing.T) {
	for _, scanError := range []bool{true, false} {
		t.Run(map[bool]string{true: "scan", false: "iteration"}[scanError], func(t *testing.T) {
			db, mock := mockDB(t)
			rows := sqlmock.NewRows([]string{"category"})
			if scanError {
				rows.AddRow(nil)
			} else {
				rows.AddRow("еда").RowError(0, errors.New("read failed"))
			}
			mock.ExpectQuery("SELECT").WillReturnRows(rows).RowsWillBeClosed()
			if _, err := NewReportRepository(db).Categories(t.Context(), "2025-09-01", "2025-09-30"); err == nil {
				t.Fatal("expected row error")
			}
		})
	}
}

func TestRepositoryErrors(t *testing.T) {
	want := errors.New("storage unavailable")
	cases := []struct {
		name string
		call func(*sql.DB) error
		exec bool
	}{
		{"upsert budget", func(db *sql.DB) error { return NewBudgetRepository(db).Upsert(t.Context(), domain.Budget{}) }, true},
		{"find budget", func(db *sql.DB) error {
			_, err := NewBudgetRepository(db).GetByCategory(t.Context(), "еда")
			return err
		}, false},
		{"list budgets", func(db *sql.DB) error { _, err := NewBudgetRepository(db).List(t.Context()); return err }, false},
		{"add transaction", func(db *sql.DB) error {
			_, err := NewTransactionRepository(db).Add(t.Context(), domain.Transaction{})
			return err
		}, false},
		{"list transactions", func(db *sql.DB) error { _, err := NewTransactionRepository(db).List(t.Context()); return err }, false},
		{"sum expenses", func(db *sql.DB) error {
			_, err := NewTransactionRepository(db).SumByCategory(t.Context(), "еда")
			return err
		}, false},
		{"report", func(db *sql.DB) error {
			_, err := NewReportRepository(db).Categories(t.Context(), "2025-09-01", "2025-09-30")
			return err
		}, false},
		{"report sum", func(db *sql.DB) error {
			_, err := NewReportRepository(db).SumByCategory(t.Context(), "еда", "2025-09-01", "2025-09-30")
			return err
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := mockDB(t)
			if tc.exec {
				mock.ExpectExec(".*").WillReturnError(want)
			} else {
				mock.ExpectQuery(".*").WillReturnError(want)
			}
			if err := tc.call(db); !errors.Is(err, want) {
				t.Fatalf("error = %v, want wrapped storage error", err)
			}
		})
	}
}

func TestBudgetListRowErrors(t *testing.T) {
	for _, scanError := range []bool{true, false} {
		t.Run(map[bool]string{true: "scan", false: "iteration"}[scanError], func(t *testing.T) {
			db, mock := mockDB(t)
			rows := sqlmock.NewRows([]string{"category", "limit_amount"})
			if scanError {
				rows.AddRow("еда", "not a number")
			} else {
				rows.AddRow("еда", 500).RowError(0, errors.New("read failed"))
			}
			mock.ExpectQuery("SELECT").WillReturnRows(rows).RowsWillBeClosed()
			if _, err := NewBudgetRepository(db).List(t.Context()); err == nil {
				t.Fatal("expected row error")
			}
		})
	}
}

func TestRepositoriesRespectCanceledContext(t *testing.T) {
	db, _ := mockDB(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := NewBudgetRepository(db).Upsert(ctx, domain.Budget{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Upsert() error = %v, want context.Canceled", err)
	}
	if _, err := NewTransactionRepository(db).List(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("List() error = %v, want context.Canceled", err)
	}
	if _, err := NewReportRepository(db).Categories(ctx, "2025-09-01", "2025-09-30"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Categories() error = %v, want context.Canceled", err)
	}
	if _, err := NewReportRepository(db).SumByCategory(ctx, "еда", "2025-09-01", "2025-09-30"); !errors.Is(err, context.Canceled) {
		t.Fatalf("SumByCategory() error = %v, want context.Canceled", err)
	}
}
