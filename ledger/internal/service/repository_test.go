package service_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"ledger/internal/domain"
	"ledger/internal/service"
)

type budgetRepository struct {
	upsert func(context.Context, domain.Budget) error
	get    func(context.Context, string) (domain.Budget, error)
	list   func(context.Context) ([]domain.Budget, error)
}

func (r budgetRepository) Upsert(ctx context.Context, budget domain.Budget) error {
	return r.upsert(ctx, budget)
}

func (r budgetRepository) GetByCategory(ctx context.Context, category string) (domain.Budget, error) {
	return r.get(ctx, category)
}

func (r budgetRepository) List(ctx context.Context) ([]domain.Budget, error) {
	return r.list(ctx)
}

type transactionRepository struct {
	add  func(context.Context, domain.Transaction) (domain.Transaction, error)
	list func(context.Context) ([]domain.Transaction, error)
	sum  func(context.Context, string) (float64, error)
}

func (r transactionRepository) Add(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	return r.add(ctx, tx)
}

func (r transactionRepository) List(ctx context.Context) ([]domain.Transaction, error) {
	return r.list(ctx)
}

func (r transactionRepository) SumByCategory(ctx context.Context, category string) (float64, error) {
	return r.sum(ctx, category)
}

type reportRepository func(context.Context, string, string) ([]domain.ReportSummary, error)

func (r reportRepository) Summary(ctx context.Context, from, to string) ([]domain.ReportSummary, error) {
	return r(ctx, from, to)
}

func TestValidationBeforeRepositoryCalls(t *testing.T) {
	// Nil repositories make any accidental storage call fail the test.
	svc := service.New(nil, nil, nil)
	requireValidationError(t, svc.SetBudget(t.Context(), domain.Budget{}), "budget limit must be positive")
	_, err := svc.AddTransaction(t.Context(), domain.Transaction{})
	requireValidationError(t, err, "transaction amount must be positive")
}

func TestTransactionRepositoryCalls(t *testing.T) {
	ctx := t.Context()
	tx := domain.Transaction{Amount: 100, Category: "еда", Description: "ланч", Date: "2025-09-10"}
	var calls []string
	svc := service.New(budgetRepository{get: func(gotCtx context.Context, category string) (domain.Budget, error) {
		if gotCtx != ctx || category != tx.Category {
			t.Fatal("budget lookup arguments were not forwarded")
		}
		calls = append(calls, "budget")
		return domain.Budget{Category: category, Limit: 500}, nil
	}}, transactionRepository{
		sum: func(gotCtx context.Context, category string) (float64, error) {
			if gotCtx != ctx || category != tx.Category {
				t.Fatal("sum arguments were not forwarded")
			}
			calls = append(calls, "sum")
			return 400, nil
		},
		add: func(gotCtx context.Context, got domain.Transaction) (domain.Transaction, error) {
			if gotCtx != ctx || got != tx {
				t.Fatal("insert arguments were not forwarded")
			}
			calls = append(calls, "add")
			got.ID = 42
			return got, nil
		},
	}, nil)
	got, err := svc.AddTransaction(ctx, tx)
	tx.ID = 42
	if err != nil || got != tx || !reflect.DeepEqual(calls, []string{"budget", "sum", "add"}) {
		t.Fatalf("AddTransaction() = %+v, %v; calls = %v", got, err, calls)
	}
}

func TestTransactionWithoutBudgetSkipsSum(t *testing.T) {
	svc := service.New(budgetRepository{get: func(context.Context, string) (domain.Budget, error) {
		return domain.Budget{}, fmt.Errorf("lookup: %w", domain.ErrBudgetNotFound)
	}}, transactionRepository{add: func(_ context.Context, tx domain.Transaction) (domain.Transaction, error) {
		tx.ID = 1
		return tx, nil
	}}, nil)
	if _, err := svc.AddTransaction(t.Context(), domain.Transaction{Amount: 100, Category: "еда"}); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionStopsOnRepositoryError(t *testing.T) {
	want := errors.New("storage failed")
	for _, stage := range []string{"budget", "sum", "add"} {
		t.Run(stage, func(t *testing.T) {
			svc := service.New(budgetRepository{get: func(context.Context, string) (domain.Budget, error) {
				if stage == "budget" {
					return domain.Budget{}, want
				}
				return domain.Budget{Category: "еда", Limit: 500}, nil
			}}, transactionRepository{
				sum: func(context.Context, string) (float64, error) {
					if stage == "budget" {
						t.Fatal("sum called after budget error")
					}
					if stage == "sum" {
						return 0, want
					}
					return 0, nil
				},
				add: func(context.Context, domain.Transaction) (domain.Transaction, error) {
					if stage != "add" {
						t.Fatal("insert called after lookup/sum error")
					}
					return domain.Transaction{}, want
				},
			}, nil)
			_, err := svc.AddTransaction(t.Context(), domain.Transaction{Amount: 100, Category: "еда"})
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want storage error", err)
			}
		})
	}
}

func TestBudgetExceededSkipsInsert(t *testing.T) {
	svc := service.New(budgetRepository{get: func(context.Context, string) (domain.Budget, error) {
		return domain.Budget{Category: "еда", Limit: 500}, nil
	}}, transactionRepository{sum: func(context.Context, string) (float64, error) { return 450, nil }}, nil)
	_, err := svc.AddTransaction(t.Context(), domain.Transaction{Amount: 100, Category: "еда"})
	if !errors.Is(err, domain.ErrBudgetExceeded) {
		t.Fatalf("error = %v, want ErrBudgetExceeded", err)
	}
}

func TestOtherRepositoryErrors(t *testing.T) {
	ctx := t.Context()
	want := errors.New("storage failed")
	budget := domain.Budget{Category: "еда", Limit: 500}
	svc := service.New(budgetRepository{
		upsert: func(gotCtx context.Context, got domain.Budget) error {
			if gotCtx != ctx || got != budget {
				t.Fatal("upsert arguments were not forwarded")
			}
			return want
		},
		list: func(gotCtx context.Context) ([]domain.Budget, error) {
			if gotCtx != ctx {
				t.Fatal("budget list context was not forwarded")
			}
			return nil, want
		},
	}, transactionRepository{list: func(gotCtx context.Context) ([]domain.Transaction, error) {
		if gotCtx != ctx {
			t.Fatal("transaction list context was not forwarded")
		}
		return nil, want
	}}, nil)
	if err := svc.SetBudget(ctx, budget); !errors.Is(err, want) {
		t.Fatalf("SetBudget error = %v", err)
	}
	if _, err := svc.ListBudgets(ctx); !errors.Is(err, want) {
		t.Fatalf("ListBudgets error = %v", err)
	}
	if _, err := svc.ListTransactions(ctx); !errors.Is(err, want) {
		t.Fatalf("ListTransactions error = %v", err)
	}
	if err := svc.LoadBudgets(ctx, strings.NewReader(`[{"category":"еда","limit":500}]`)); !errors.Is(err, want) {
		t.Fatalf("LoadBudgets error = %v", err)
	}
}

func TestGetReportSummary(t *testing.T) {
	want := []domain.ReportSummary{{Category: "еда", Amount: 100}}
	svc := service.New(nil, nil, reportRepository(func(ctx context.Context, from, to string) ([]domain.ReportSummary, error) {
		if ctx != t.Context() || from != "2025-09-01" || to != "2025-09-30" {
			t.Fatal("report arguments were not forwarded")
		}
		return want, nil
	}))
	got, err := svc.GetReportSummary(t.Context(), "2025-09-01", "2025-09-30")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("GetReportSummary() = %+v, %v; want %+v", got, err, want)
	}
	storageErr := errors.New("report failed")
	svc = service.New(nil, nil, reportRepository(func(context.Context, string, string) ([]domain.ReportSummary, error) {
		return nil, storageErr
	}))
	if _, err := svc.GetReportSummary(t.Context(), "2025-09-01", "2025-09-01"); !errors.Is(err, storageErr) {
		t.Fatalf("error = %v, want storage error", err)
	}
}

func TestGetReportSummaryRejectsInvalidDates(t *testing.T) {
	for _, tc := range []struct{ name, from, to, want string }{
		{"invalid from", "bad", "2025-09-30", "invalid from date:"},
		{"invalid to", "2025-09-01", "bad", "invalid to date:"},
		{"reversed range", "2025-09-30", "2025-09-01", "from and to must be valid dates with from <= to"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := service.New(nil, nil, nil)
			_, err := svc.GetReportSummary(t.Context(), tc.from, tc.to)
			var validationErr *service.ValidationError
			if !errors.As(err, &validationErr) || !strings.HasPrefix(err.Error(), tc.want) {
				t.Fatalf("error = %v, want validation error starting with %q", err, tc.want)
			}
		})
	}
}

func TestValidationErrorUnwrap(t *testing.T) {
	want := errors.New("invalid input")
	err := &service.ValidationError{Err: want}
	if !errors.Is(err, want) || err.Error() != want.Error() {
		t.Fatalf("ValidationError did not preserve the original error: %v", err)
	}
}
