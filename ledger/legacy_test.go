package ledger

import (
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLegacyMemoryAPI(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	if err := LoadBudgets(strings.NewReader(`[{"category":"еда","limit":500}]`)); err != nil {
		t.Fatal(err)
	}
	budgets, err := ListBudgets()
	if err != nil || len(budgets) != 1 {
		t.Fatalf("ListBudgets() = %+v, %v", budgets, err)
	}
	if err := AddTransaction(Transaction{Amount: 500, Category: "еда"}); err != nil {
		t.Fatal(err)
	}
	if err := AddTransaction(Transaction{Amount: 1, Category: "еда"}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("error = %v, want ErrBudgetExceeded", err)
	}
	if err := CheckValid(Budget{}); err == nil || err.Error() != "validation failed: budget limit must be positive" {
		t.Fatalf("CheckValid() error = %v", err)
	}
	Reset()
	transactions, err := ListTransactions()
	if err != nil || transactions == nil || len(transactions) != 0 {
		t.Fatalf("ListTransactions() after Reset = %+v, %v", transactions, err)
	}
}

func TestLegacyPostgresAPI(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	previousDB, previousRedis := DB, redisClient
	SetDB(db)
	SetRedisClient(nil)
	t.Cleanup(func() {
		SetDB(previousDB)
		SetRedisClient(previousRedis)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		_ = db.Close()
	})
	mock.ExpectExec("INSERT INTO budgets").WithArgs("еда", 500.0).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := SetBudget(Budget{Category: "еда", Limit: 500}); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT limit_amount").WithArgs("еда").
		WillReturnRows(sqlmock.NewRows([]string{"limit_amount"}).AddRow(500))
	mock.ExpectQuery("SELECT COALESCE").WithArgs("еда").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(400))
	mock.ExpectQuery("INSERT INTO expenses").WithArgs(100.0, "еда", "", "2025-09-10").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	tx, err := AddTransactionAndReturn(Transaction{Amount: 100, Category: "еда", Date: "2025-09-10"})
	if err != nil || tx.ID != 42 {
		t.Fatalf("AddTransactionAndReturn() = %+v, %v", tx, err)
	}
	mock.ExpectQuery("SELECT category, COALESCE").WithArgs("2025-09-01", "2025-09-30").
		WillReturnRows(sqlmock.NewRows([]string{"category", "amount"}).AddRow("еда", 500))
	report, err := GetReportSummary(t.Context(), "2025-09-01", "2025-09-30")
	if err != nil || len(report) != 1 || report[0].Amount != 500 {
		t.Fatalf("GetReportSummary() = %+v, %v", report, err)
	}
}
