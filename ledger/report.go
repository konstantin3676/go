package ledger

import (
	"context"
	"fmt"
	"time"
)

type ReportSummary struct {
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
}

func GetReportSummary(ctx context.Context, from, to string) ([]ReportSummary, error) {
	if _, err := time.Parse("2006-01-02", from); err != nil {
		return nil, fmt.Errorf("invalid from date: %w", err)
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		return nil, fmt.Errorf("invalid to date: %w", err)
	}

	key := fmt.Sprintf("report:summary:%s:%s", from, to)
	if redisClient != nil {
		if cached, found, err := readReportCache(ctx, key); err != nil {
			return nil, err
		} else if found {
			return cached, nil
		}
	}

	if DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	rows, err := DB.QueryContext(ctx, `
		SELECT category, COALESCE(SUM(amount), 0)
		FROM expenses
		WHERE date BETWEEN $1 AND $2
		GROUP BY category
		ORDER BY category`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query report summary: %w", err)
	}
	defer rows.Close()

	result := make([]ReportSummary, 0)
	for rows.Next() {
		var item ReportSummary
		if err := rows.Scan(&item.Category, &item.Amount); err != nil {
			return nil, fmt.Errorf("scan report summary: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report summary: %w", err)
	}

	if redisClient != nil {
		if err := writeReportCache(ctx, key, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}
