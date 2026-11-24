package service_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"

	"ledger/internal/domain"
	"ledger/internal/service"
)

func TestImportTransactionsMixed(t *testing.T) {
	svc := newService()
	if err := svc.SetBudget(t.Context(), domain.Budget{Category: "food", Limit: 10}); err != nil {
		t.Fatal(err)
	}
	summary, err := svc.ImportTransactions(t.Context(), []domain.Transaction{
		{Amount: 8, Category: "food"},
		{Amount: -1, Category: "food"},
		{Amount: 2, Category: "books"},
		{Amount: 11, Category: "food"},
	}, 4)
	want := service.ImportSummary{Accepted: 2, Rejected: 2, Errors: []service.ImportError{
		{Index: 1, Error: "transaction amount must be positive"},
		{Index: 3, Error: `budget exceeded for category "food"`},
	}}
	if err != nil || !reflect.DeepEqual(summary, want) {
		t.Fatalf("import = %+v, %v; want %+v", summary, err, want)
	}
}

func TestImportTransactionsBudgetUnderContention(t *testing.T) {
	svc := newService()
	if err := svc.SetBudget(t.Context(), domain.Budget{Category: "food", Limit: 10}); err != nil {
		t.Fatal(err)
	}
	batch := make([]domain.Transaction, 100)
	for i := range batch {
		batch[i] = domain.Transaction{Amount: 1, Category: "food"}
	}
	summary, err := svc.ImportTransactions(t.Context(), batch, 16)
	if err != nil || summary.Accepted != 10 || summary.Rejected != 90 || len(summary.Errors) != 90 {
		t.Fatalf("import counts: accepted = %d, rejected = %d, errors = %d; err = %v", summary.Accepted, summary.Rejected, len(summary.Errors), err)
	}
	previous := -1
	for _, item := range summary.Errors {
		if item.Index <= previous || item.Index >= len(batch) || !strings.Contains(item.Error, domain.ErrBudgetExceeded.Error()) {
			t.Fatalf("invalid or unordered rejection: %+v (previous index: %d)", item, previous)
		}
		previous = item.Index
	}
	saved, err := svc.ListTransactions(t.Context())
	if err != nil || len(saved) != 10 {
		t.Fatalf("saved = %d, %v; want 10", len(saved), err)
	}
}

func TestImportTransactionsCanceledWithPartialSummary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		insertStarted := make(chan struct{})
		calls := 0
		svc := service.New(budgetRepository{get: withoutBudget}, transactionRepository{
			add: func(gotCtx context.Context, tx domain.Transaction) (domain.Transaction, error) {
				if gotCtx != ctx {
					t.Error("request context was not forwarded to insert")
					return domain.Transaction{}, errors.New("context not forwarded")
				}
				calls++
				if calls == 2 {
					close(insertStarted)
					<-ctx.Done()
					return domain.Transaction{}, ctx.Err()
				}
				return tx, nil
			},
		}, nil)
		batch := []domain.Transaction{
			{Amount: -1, Category: "food"},
			{Amount: 1, Category: "food"},
			{Amount: 1, Category: "food"},
			{Amount: 1, Category: "food"},
		}
		go func() {
			<-insertStarted
			cancel()
		}()
		summary, err := svc.ImportTransactions(ctx, batch, 1)
		want := service.ImportSummary{Accepted: 1, Rejected: 1, Errors: []service.ImportError{
			{Index: 0, Error: "transaction amount must be positive"},
		}}
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(summary, want) || calls != 2 {
			t.Fatalf("import = %+v, %v, %d insert calls; want %+v, context.Canceled, 2 calls", summary, err, calls, want)
		}
	})
}

func TestImportTransactionsProcessesDifferentCategoriesConcurrently(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered := make(chan string, 2)
		release := make(chan struct{})
		done := make(chan struct{})
		svc := service.New(budgetRepository{get: withoutBudget}, transactionRepository{
			add: func(_ context.Context, tx domain.Transaction) (domain.Transaction, error) {
				entered <- tx.Category
				<-release
				return tx, nil
			},
		}, nil)
		go func() {
			defer close(done)
			summary, err := svc.ImportTransactions(t.Context(), []domain.Transaction{
				{Amount: 1, Category: "food"}, {Amount: 1, Category: "books"},
			}, 2)
			if err != nil || summary.Accepted != 2 {
				t.Errorf("import = %+v, %v; want both accepted", summary, err)
			}
		}()
		synctest.Wait()
		if n := len(entered); n != 2 {
			t.Errorf("%d workers reached the repository; want 2 while both inserts are blocked", n)
		}
		close(release)
		<-done
	})
}

func TestImportTransactionsCancelsWhileWaitingForSingleInsert(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		release := make(chan struct{})
		started := make(chan struct{})
		singleDone := make(chan error, 1)
		svc := service.New(budgetRepository{get: withoutBudget}, transactionRepository{
			add: func(_ context.Context, tx domain.Transaction) (domain.Transaction, error) {
				if tx.Description == "single" {
					close(started)
					<-release
				} else {
					t.Error("bulk insert entered the same category while the single insert held its lock")
				}
				return tx, nil
			},
		}, nil)
		go func() {
			_, err := svc.AddTransaction(t.Context(), domain.Transaction{Amount: 1, Category: "food", Description: "single"})
			singleDone <- err
		}()
		<-started
		bulkDone := make(chan struct{})
		go func() {
			defer close(bulkDone)
			summary, err := svc.ImportTransactions(ctx, []domain.Transaction{{Amount: 1, Category: "food"}}, 1)
			if !errors.Is(err, context.Canceled) || summary.Accepted != 0 || summary.Rejected != 0 {
				t.Errorf("import = %+v, %v; want no completed transactions and context.Canceled", summary, err)
			}
		}()
		synctest.Wait()
		cancel()
		synctest.Wait()
		select {
		case <-bulkDone:
		default:
			t.Error("bulk import did not stop while waiting for the category lock")
		}
		close(release)
		if err := <-singleDone; err != nil {
			t.Errorf("single insert failed: %v", err)
		}
		<-bulkDone
	})
}

func TestImportTransactionsEmpty(t *testing.T) {
	svc := service.New(nil, nil, nil)
	summary, err := svc.ImportTransactions(t.Context(), nil, 4)
	if err != nil || !reflect.DeepEqual(summary, service.ImportSummary{Errors: []service.ImportError{}}) {
		t.Fatalf("empty import = %+v, %v", summary, err)
	}
	_, err = svc.ImportTransactions(t.Context(), nil, 0)
	requireValidationError(t, err, "workers must be positive")
}

func withoutBudget(context.Context, string) (domain.Budget, error) {
	return domain.Budget{}, domain.ErrBudgetNotFound
}
