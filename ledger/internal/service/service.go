package service

import (
	"context"
	"io"

	"ledger/internal/domain"
)

type Service interface {
	SetBudget(context.Context, domain.Budget) error
	ListBudgets(context.Context) ([]domain.Budget, error)
	AddTransaction(context.Context, domain.Transaction) (domain.Transaction, error)
	ImportTransactions(context.Context, []domain.Transaction, int) (ImportSummary, error)
	ListTransactions(context.Context) ([]domain.Transaction, error)
	GetReportSummary(context.Context, string, string) ([]domain.ReportSummary, error)
	LoadBudgets(context.Context, io.Reader) error
}

type service struct {
	budgets       domain.BudgetRepository
	transactions  domain.TransactionRepository
	reports       domain.ReportRepository
	categoryLocks categoryLocks
}

var _ Service = (*service)(nil)

func New(budgets domain.BudgetRepository, transactions domain.TransactionRepository, reports domain.ReportRepository) Service {
	return &service{budgets: budgets, transactions: transactions, reports: reports}
}
