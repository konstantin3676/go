package service_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"ledger/internal/domain"
	"ledger/internal/repository/memory"
	"ledger/internal/service"
)

func newService() service.Service {
	store := memory.NewStore()
	return service.New(memory.NewBudgetRepository(store), memory.NewTransactionRepository(store), nil)
}

func TestSetBudgetAddsAndUpdatesBudget(t *testing.T) {
	svc := newService()
	for _, limit := range []float64{1000, 1500} {
		if err := svc.SetBudget(t.Context(), domain.Budget{Category: "Продукты", Limit: limit}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := svc.ListBudgets(t.Context())
	want := []domain.Budget{{Category: "Продукты", Limit: 1500}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ListBudgets() = %+v, %v; want %+v", got, err, want)
	}
	got[0].Limit = 999
	current, err := svc.ListBudgets(t.Context())
	if err != nil || !reflect.DeepEqual(current, want) {
		t.Fatal("ListBudgets returned internal storage instead of a copy", err)
	}
}

func TestSetBudgetRejectsInvalidBudget(t *testing.T) {
	cases := []struct {
		name   string
		budget domain.Budget
		want   string
	}{
		{"zero limit", domain.Budget{Category: "Продукты"}, "budget limit must be positive"},
		{"negative limit", domain.Budget{Category: "Продукты", Limit: -100}, "budget limit must be positive"},
		{"empty category", domain.Budget{Limit: 1000}, "budget category must not be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService()
			requireValidationError(t, svc.SetBudget(t.Context(), tc.budget), tc.want)
			budgets, err := svc.ListBudgets(t.Context())
			if err != nil || len(budgets) != 0 {
				t.Fatalf("invalid budget was saved: %+v, %v", budgets, err)
			}
		})
	}
}

func TestAddTransactionChecksBudget(t *testing.T) {
	svc := newService()
	if err := svc.SetBudget(t.Context(), domain.Budget{Category: "Продукты", Limit: 500}); err != nil {
		t.Fatal(err)
	}
	for _, amount := range []float64{200, 300} {
		if _, err := svc.AddTransaction(t.Context(), domain.Transaction{Amount: amount, Category: "Продукты"}); err != nil {
			t.Fatalf("transaction within budget returned an error: %v", err)
		}
	}
	_, err := svc.AddTransaction(t.Context(), domain.Transaction{Amount: 1, Category: "Продукты"})
	if !errors.Is(err, domain.ErrBudgetExceeded) {
		t.Fatalf("error = %v, want ErrBudgetExceeded", err)
	}
	transactions, err := svc.ListTransactions(t.Context())
	if err != nil || len(transactions) != 2 {
		t.Fatalf("transaction exceeding budget was saved: %+v, %v", transactions, err)
	}
}

func TestAddTransactionWithoutBudget(t *testing.T) {
	svc := newService()
	tx := domain.Transaction{Amount: 10000, Category: "Образование"}
	got, err := svc.AddTransaction(t.Context(), tx)
	tx.ID = 1
	if err != nil || got != tx {
		t.Fatalf("AddTransaction() = %+v, %v; want %+v", got, err, tx)
	}
}

func TestAddAndListTransactions(t *testing.T) {
	svc := newService()
	first := domain.Transaction{Amount: 100, Category: "Продукты"}
	second := domain.Transaction{Amount: 250, Category: "Транспорт"}
	for _, tx := range []domain.Transaction{first, second} {
		if _, err := svc.AddTransaction(t.Context(), tx); err != nil {
			t.Fatal(err)
		}
	}
	first.ID, second.ID = 1, 2
	want := []domain.Transaction{second, first}
	got, err := svc.ListTransactions(t.Context())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ListTransactions() = %+v, %v; want %+v", got, err, want)
	}
	got[0].Amount = 999
	current, err := svc.ListTransactions(t.Context())
	if err != nil || !reflect.DeepEqual(current, want) {
		t.Fatal("ListTransactions returned internal storage instead of a copy", err)
	}
}

func TestAddTransactionRejectsInvalidTransaction(t *testing.T) {
	cases := []struct {
		name string
		tx   domain.Transaction
		want string
	}{
		{"zero amount", domain.Transaction{Category: "Продукты"}, "transaction amount must be positive"},
		{"negative amount", domain.Transaction{Amount: -50, Category: "Продукты"}, "transaction amount must be positive"},
		{"empty category", domain.Transaction{Amount: 100}, "transaction category must not be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService()
			_, err := svc.AddTransaction(t.Context(), tc.tx)
			requireValidationError(t, err, tc.want)
			transactions, err := svc.ListTransactions(t.Context())
			if err != nil || len(transactions) != 0 {
				t.Fatalf("invalid transaction was saved: %+v, %v", transactions, err)
			}
		})
	}
}

func TestLoadBudgets(t *testing.T) {
	svc := newService()
	input := `[{"category":"Продукты","limit":5000},{"category":"Транспорт","limit":1000}]`
	if err := svc.LoadBudgets(t.Context(), strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ListBudgets(t.Context())
	want := []domain.Budget{{Category: "Продукты", Limit: 5000}, {Category: "Транспорт", Limit: 1000}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ListBudgets() = %+v, %v; want %+v", got, err, want)
	}
}

func TestLoadBudgetsRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"invalid JSON", `[{"category":`, "could not parse budgets JSON"},
		{"invalid budget", `[{"category":"","limit":1000}]`, "could not set budget"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService()
			err := svc.LoadBudgets(t.Context(), strings.NewReader(tc.input))
			var validationErr *service.ValidationError
			if !errors.As(err, &validationErr) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want validation error containing %q", err, tc.want)
			}
		})
	}
}

func TestCheckValid(t *testing.T) {
	for _, valid := range []domain.Validatable{
		domain.Transaction{Amount: 100, Category: "Продукты"},
		domain.Budget{Category: "Продукты", Limit: 500},
	} {
		if err := service.CheckValid(valid); err != nil {
			t.Fatal(err)
		}
	}
	requireValidationError(t, service.CheckValid(domain.Transaction{}), "validation failed: transaction amount must be positive")
	requireValidationError(t, service.CheckValid(domain.Budget{}), "validation failed: budget limit must be positive")
}

func TestConcurrentTransactionsRespectBudget(t *testing.T) {
	svc := newService()
	if err := svc.SetBudget(t.Context(), domain.Budget{Category: "еда", Limit: 100}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_, err := svc.AddTransaction(t.Context(), domain.Transaction{Amount: 10, Category: "еда"})
			if err != nil && !errors.Is(err, domain.ErrBudgetExceeded) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	wg.Wait()
	transactions, err := svc.ListTransactions(t.Context())
	if err != nil || len(transactions) != 10 {
		t.Fatalf("got %d transactions, %v; want 10", len(transactions), err)
	}
}

func TestIndependentServices(t *testing.T) {
	first, second := newService(), newService()
	if err := first.SetBudget(t.Context(), domain.Budget{Category: "еда", Limit: 100}); err != nil {
		t.Fatal(err)
	}
	got, err := second.ListBudgets(t.Context())
	if err != nil || len(got) != 0 {
		t.Fatalf("service state is not isolated: %+v, %v", got, err)
	}
}

func requireValidationError(t *testing.T, err error, want string) {
	t.Helper()
	var validationErr *service.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want *ValidationError", err)
	}
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestCanceledContext(t *testing.T) {
	svc := newService()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := svc.AddTransaction(ctx, domain.Transaction{Amount: 100, Category: "еда"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
