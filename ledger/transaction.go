package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
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

	var limit float64
	err := DB.QueryRowContext(context.Background(),
		`SELECT limit_amount FROM budgets WHERE category = $1`, tx.Category).Scan(&limit)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, fmt.Errorf("find budget: %w", err)
	}
	if err == nil {
		var spent float64
		if err := DB.QueryRowContext(context.Background(),
			`SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE category = $1`, tx.Category).Scan(&spent); err != nil {
			return Transaction{}, fmt.Errorf("calculate spent amount: %w", err)
		}
		if spent+tx.Amount > limit {
			return Transaction{}, fmt.Errorf("%w for category %q", ErrBudgetExceeded, tx.Category)
		}
	}

	err = DB.QueryRowContext(context.Background(), `
		INSERT INTO expenses(amount, category, description, date)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, tx.Amount, tx.Category, tx.Description, tx.Date).Scan(&tx.ID)
	if err != nil {
		return Transaction{}, fmt.Errorf("insert transaction: %w", err)
	}
	return tx, nil
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

	rows, err := DB.Query(`
		SELECT id, amount, category, COALESCE(description, ''), date::text
		FROM expenses
		ORDER BY date DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	result := make([]Transaction, 0)
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.ID, &tx.Amount, &tx.Category, &tx.Description, &tx.Date); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		result = append(result, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactions: %w", err)
	}
	return result, nil
}
