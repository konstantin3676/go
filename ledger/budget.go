package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"

	"ledger/internal/repository/cached"
	"ledger/internal/repository/pg"
)

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

	repo := cached.NewBudgetRepository(pg.NewBudgetRepository(DB), redisClient)
	return repo.Upsert(context.Background(), b)
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

	repo := cached.NewBudgetRepository(pg.NewBudgetRepository(DB), redisClient)
	return repo.List(context.Background())
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
