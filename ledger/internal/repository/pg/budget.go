package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ledger/internal/domain"
)

type BudgetRepository struct {
	db *sql.DB
}

var _ domain.BudgetRepository = (*BudgetRepository)(nil)

func NewBudgetRepository(db *sql.DB) *BudgetRepository {
	return &BudgetRepository{db: db}
}

func (r *BudgetRepository) Upsert(ctx context.Context, budget domain.Budget) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO budgets(category, limit_amount)
		VALUES ($1, $2)
		ON CONFLICT (category)
		DO UPDATE SET limit_amount = EXCLUDED.limit_amount`, budget.Category, budget.Limit)
	if err != nil {
		return fmt.Errorf("upsert budget: %w", err)
	}
	return nil
}

func (r *BudgetRepository) GetByCategory(ctx context.Context, category string) (domain.Budget, error) {
	budget := domain.Budget{Category: category}
	err := r.db.QueryRowContext(ctx,
		`SELECT limit_amount FROM budgets WHERE category = $1`, category).Scan(&budget.Limit)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Budget{}, domain.ErrBudgetNotFound
	}
	if err != nil {
		return domain.Budget{}, fmt.Errorf("find budget: %w", err)
	}
	return budget, nil
}

func (r *BudgetRepository) List(ctx context.Context) ([]domain.Budget, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT category, limit_amount FROM budgets ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()

	result := make([]domain.Budget, 0)
	for rows.Next() {
		var budget domain.Budget
		if err := rows.Scan(&budget.Category, &budget.Limit); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		result = append(result, budget)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate budgets: %w", err)
	}
	return result, nil
}
