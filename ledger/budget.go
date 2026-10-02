package main

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

var budgets = map[string]Budget{}

func SetBudget(b Budget) {
	budgets[b.Category] = b
}

func LoadBudgets(r io.Reader) error {
	var loadedBudgets []Budget
	if err := json.NewDecoder(r).Decode(&loadedBudgets); err != nil {
		return fmt.Errorf("could not parse budgets JSON: %w", err)
	}

	for _, budget := range loadedBudgets {
		SetBudget(budget)
	}

	return nil
}
