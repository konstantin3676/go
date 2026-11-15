package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"ledger/internal/domain"
)

func (s *service) SetBudget(ctx context.Context, budget domain.Budget) error {
	if err := budget.Validate(); err != nil {
		return &ValidationError{Err: err}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.budgets.Upsert(ctx, budget)
}

func (s *service) ListBudgets(ctx context.Context) ([]domain.Budget, error) {
	return s.budgets.List(ctx)
}

func (s *service) LoadBudgets(ctx context.Context, r io.Reader) error {
	var budgets []domain.Budget
	if err := json.NewDecoder(r).Decode(&budgets); err != nil {
		return &ValidationError{Err: fmt.Errorf("could not parse budgets JSON: %w", err)}
	}
	for _, budget := range budgets {
		if err := s.SetBudget(ctx, budget); err != nil {
			return fmt.Errorf("could not set budget: %w", err)
		}
	}
	return nil
}
