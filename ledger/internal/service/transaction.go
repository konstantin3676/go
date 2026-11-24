package service

import (
	"context"
	"errors"
	"fmt"

	"ledger/internal/domain"
)

func (s *service) AddTransaction(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	if err := tx.Validate(); err != nil {
		return domain.Transaction{}, &ValidationError{Err: err}
	}
	release, err := s.categoryLocks.acquire(ctx, tx.Category)
	if err != nil {
		return domain.Transaction{}, err
	}
	defer release()

	budget, err := s.budgets.GetByCategory(ctx, tx.Category)
	if err != nil && !errors.Is(err, domain.ErrBudgetNotFound) {
		return domain.Transaction{}, err
	}
	if err == nil {
		spent, err := s.transactions.SumByCategory(ctx, tx.Category)
		if err != nil {
			return domain.Transaction{}, err
		}
		if spent+tx.Amount > budget.Limit {
			return domain.Transaction{}, fmt.Errorf("%w for category %q", domain.ErrBudgetExceeded, tx.Category)
		}
	}
	return s.transactions.Add(ctx, tx)
}

func (s *service) ListTransactions(ctx context.Context) ([]domain.Transaction, error) {
	return s.transactions.List(ctx)
}
