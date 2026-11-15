package pg

import (
	"context"
	"database/sql"
	"fmt"

	"ledger/internal/domain"
)

type ReportRepository struct {
	db *sql.DB
}

var _ domain.ReportDataRepository = (*ReportRepository)(nil)

func NewReportRepository(db *sql.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

func (r *ReportRepository) Categories(ctx context.Context, from, to string) ([]string, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT category
		FROM expenses
		WHERE date BETWEEN $1 AND $2
		ORDER BY category`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query report categories: %w", err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf("scan report category: %w", err)
		}
		result = append(result, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report categories: %w", err)
	}
	return result, nil
}

func (r *ReportRepository) SumByCategory(ctx context.Context, category, from, to string) (float64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database is not initialized")
	}
	var amount float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM expenses
		WHERE category = $1 AND date BETWEEN $2 AND $3`, category, from, to).Scan(&amount)
	if err != nil {
		return 0, fmt.Errorf("sum report category %q: %w", category, err)
	}
	return amount, nil
}
