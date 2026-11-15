package domain

import "context"

type BudgetRepository interface {
	Upsert(ctx context.Context, budget Budget) error
	GetByCategory(ctx context.Context, category string) (Budget, error)
	List(ctx context.Context) ([]Budget, error)
}

type TransactionRepository interface {
	Add(ctx context.Context, transaction Transaction) (Transaction, error)
	List(ctx context.Context) ([]Transaction, error)
	SumByCategory(ctx context.Context, category string) (float64, error)
}

type ReportRepository interface {
	Summary(ctx context.Context, from, to string) ([]ReportSummary, error)
}

// ReportDataRepository provides independent queries for the report calculator.
// Both dates are inclusive and use the YYYY-MM-DD format.
type ReportDataRepository interface {
	Categories(ctx context.Context, from, to string) ([]string, error)
	SumByCategory(ctx context.Context, category, from, to string) (float64, error)
}
