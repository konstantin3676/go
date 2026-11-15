package ledger

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"ledger/internal/repository/pg"
)

var transactions = []Transaction{}

func AddTransaction(tx Transaction) error {
	_, err := AddTransactionAndReturn(tx)
	return err
}

// AddTransactionAndReturn is used by HTTP handlers that need to include the
// database-generated ID in their response. AddTransaction remains the simple
// error-only API used by the domain layer.
func AddTransactionAndReturn(tx Transaction) (Transaction, error) {
	if err := tx.Validate(); err != nil {
		return Transaction{}, err
	}

	if DB == nil {
		stateMu.Lock()
		if budget, exists := budgets[tx.Category]; exists {
			var spent float64
			for _, savedTransaction := range transactions {
				if savedTransaction.Category == tx.Category {
					spent += savedTransaction.Amount
				}
			}
			if spent+tx.Amount > budget.Limit {
				stateMu.Unlock()
				return Transaction{}, fmt.Errorf("%w for category %q", ErrBudgetExceeded, tx.Category)
			}
		}
		tx.ID = len(transactions) + 1
		transactions = append(transactions, tx)
		stateMu.Unlock()
		return tx, nil
	}

	ctx := context.Background()
	budget, err := pg.NewBudgetRepository(DB).GetByCategory(ctx, tx.Category)
	if err != nil && !errors.Is(err, ErrBudgetNotFound) {
		return Transaction{}, err
	}
	repo := pg.NewTransactionRepository(DB)
	if err == nil {
		spent, err := repo.SumByCategory(ctx, tx.Category)
		if err != nil {
			return Transaction{}, err
		}
		if spent+tx.Amount > budget.Limit {
			return Transaction{}, fmt.Errorf("%w for category %q", ErrBudgetExceeded, tx.Category)
		}
	}

	return repo.Add(ctx, tx)
}

func ListTransactions() ([]Transaction, error) {
	if DB == nil {
		stateMu.RLock()
		result := make([]Transaction, len(transactions))
		copy(result, transactions)
		stateMu.RUnlock()
		sort.SliceStable(result, func(i, j int) bool {
			if result[i].Date == result[j].Date {
				return result[i].ID > result[j].ID
			}
			return result[i].Date > result[j].Date
		})
		return result, nil
	}

	return pg.NewTransactionRepository(DB).List(context.Background())
}
