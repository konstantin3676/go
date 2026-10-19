package ledger

import (
	"errors"
	"fmt"
)

type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

func (t Transaction) Validate() error {
	if t.Amount <= 0 {
		return errors.New("transaction amount must be positive")
	}
	if t.Category == "" {
		return errors.New("transaction category must not be empty")
	}
	return nil
}

var transactions = []Transaction{}

func AddTransaction(tx Transaction) error {
	if err := tx.Validate(); err != nil {
		return err
	}

	if budget, exists := budgets[tx.Category]; exists {
		var spent float64
		for _, savedTransaction := range transactions {
			if savedTransaction.Category == tx.Category {
				spent += savedTransaction.Amount
			}
		}

		if spent+tx.Amount > budget.Limit {
			return fmt.Errorf("%w for category %q", ErrBudgetExceeded, tx.Category)
		}
	}

	tx.ID = len(transactions) + 1
	transactions = append(transactions, tx)

	return nil
}

func ListTransactions() []Transaction {
	result := make([]Transaction, len(transactions))
	copy(result, transactions)
	return result
}
