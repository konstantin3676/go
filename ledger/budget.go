package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
)

var ErrBudgetExceeded = errors.New("budget exceeded")

type Budget struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

func (b Budget) Validate() error {
	if b.Limit <= 0 {
		return errors.New("budget limit must be positive")
	}
	if b.Category == "" {
		return errors.New("budget category must not be empty")
	}
	return nil
}

var (
	budgets = map[string]Budget{}
	stateMu sync.RWMutex
)

func SetBudget(b Budget) error {
	if err := b.Validate(); err != nil {
		return err
	}

	if DB == nil {
		stateMu.Lock()
		budgets[b.Category] = b
		stateMu.Unlock()
		return nil
	}

	_, err := DB.ExecContext(context.Background(), `
		INSERT INTO budgets(category, limit_amount)
		VALUES ($1, $2)
		ON CONFLICT (category)
		DO UPDATE SET limit_amount = EXCLUDED.limit_amount`, b.Category, b.Limit)
	if err != nil {
		return fmt.Errorf("upsert budget: %w", err)
	}
	if err := invalidateBudgetsCache(context.Background()); err != nil {
		return err
	}
	return nil
}

func ListBudgets() ([]Budget, error) {
	if DB == nil {
		stateMu.RLock()
		result := make([]Budget, 0, len(budgets))
		for _, budget := range budgets {
			result = append(result, budget)
		}
		stateMu.RUnlock()
		sort.Slice(result, func(i, j int) bool {
			return result[i].Category < result[j].Category
		})
		return result, nil
	}

	rows, err := DB.Query(`SELECT category, limit_amount FROM budgets ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()

	result := make([]Budget, 0)
	for rows.Next() {
		var budget Budget
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

func LoadBudgets(r io.Reader) error {
	var loadedBudgets []Budget
	if err := json.NewDecoder(r).Decode(&loadedBudgets); err != nil {
		return fmt.Errorf("could not parse budgets JSON: %w", err)
	}

	for _, budget := range loadedBudgets {
		if err := SetBudget(budget); err != nil {
			return fmt.Errorf("could not set budget: %w", err)
		}
	}

	return nil
}
