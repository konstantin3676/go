package main

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

var transactions = []Transaction{}

func AddTransaction(tx Transaction) error {
	if tx.Amount == 0 {
		return errors.New("transaction amount must not be zero")
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
