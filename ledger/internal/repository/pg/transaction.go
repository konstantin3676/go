package pg

import (
	"context"
	"database/sql"
	"fmt"

	"ledger/internal/domain"
)

type TransactionRepository struct {
	db *sql.DB
}

var _ domain.TransactionRepository = (*TransactionRepository)(nil)

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Add(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO expenses(amount, category, description, date)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, tx.Amount, tx.Category, tx.Description, tx.Date).Scan(&tx.ID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("insert transaction: %w", err)
	}
	return tx, nil
}

func (r *TransactionRepository) List(ctx context.Context) ([]domain.Transaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, amount, category, COALESCE(description, ''), date::text
		FROM expenses
		ORDER BY date DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	result := make([]domain.Transaction, 0)
	for rows.Next() {
		var tx domain.Transaction
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

func (r *TransactionRepository) SumByCategory(ctx context.Context, category string) (float64, error) {
	var sum float64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE category = $1`, category).Scan(&sum)
	if err != nil {
		return 0, fmt.Errorf("calculate spent amount: %w", err)
	}
	return sum, nil
}
