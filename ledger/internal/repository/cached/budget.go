package cached

import (
	"context"
	"fmt"
	"time"

	"ledger/internal/domain"

	"github.com/redis/go-redis/v9"
)

const budgetsKey = "budgets:all"

type BudgetRepository struct {
	source domain.BudgetRepository
	client *redis.Client
}

var _ domain.BudgetRepository = (*BudgetRepository)(nil)

func NewBudgetRepository(source domain.BudgetRepository, client *redis.Client) *BudgetRepository {
	return &BudgetRepository{source: source, client: client}
}

func (r *BudgetRepository) Upsert(ctx context.Context, budget domain.Budget) error {
	if err := r.source.Upsert(ctx, budget); err != nil {
		return err
	}
	if r.client != nil {
		if err := r.client.Del(ctx, budgetsKey).Err(); err != nil {
			return fmt.Errorf("invalidate budgets cache: %w", err)
		}
	}
	return nil
}

func (r *BudgetRepository) GetByCategory(ctx context.Context, category string) (domain.Budget, error) {
	return r.source.GetByCategory(ctx, category)
}

func (r *BudgetRepository) List(ctx context.Context) ([]domain.Budget, error) {
	if result, found, err := readJSON[[]domain.Budget](ctx, r.client, budgetsKey, "budgets"); err != nil {
		return nil, err
	} else if found {
		return result, nil
	}
	result, err := r.source.List(ctx)
	if err != nil {
		return nil, err
	}
	if err := writeJSON(ctx, r.client, budgetsKey, "budgets", result, 20*time.Second); err != nil {
		return nil, err
	}
	return result, nil
}
