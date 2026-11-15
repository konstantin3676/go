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

var _ domain.ReportRepository = (*ReportRepository)(nil)

func NewReportRepository(db *sql.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

func (r *ReportRepository) Summary(ctx context.Context, from, to string) ([]domain.ReportSummary, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT category, COALESCE(SUM(amount), 0)
		FROM expenses
		WHERE date BETWEEN $1 AND $2
		GROUP BY category
		ORDER BY category`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query report summary: %w", err)
	}
	defer rows.Close()

	result := make([]domain.ReportSummary, 0)
	for rows.Next() {
		var item domain.ReportSummary
		if err := rows.Scan(&item.Category, &item.Amount); err != nil {
			return nil, fmt.Errorf("scan report summary: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report summary: %w", err)
	}
	return result, nil
}
