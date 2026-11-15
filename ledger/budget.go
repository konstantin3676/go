package ledger

import (
	"context"
	"io"
)

func SetBudget(b Budget) error {
	return legacyService().SetBudget(context.Background(), b)
}

func ListBudgets() ([]Budget, error) {
	return legacyService().ListBudgets(context.Background())
}

func LoadBudgets(r io.Reader) error {
	return legacyService().LoadBudgets(context.Background(), r)
}
