package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

var budgets = map[string]Budget{}

func SetBudget(b Budget) error {
	if err := b.Validate(); err != nil {
		return err
	}
	budgets[b.Category] = b
	return nil
}

func ListBudgets() []Budget {
	result := make([]Budget, 0, len(budgets))
	for _, budget := range budgets {
		result = append(result, budget)
	}
	return result
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
