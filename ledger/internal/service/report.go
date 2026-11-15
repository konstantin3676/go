package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ledger/internal/domain"
)

func (s *service) GetReportSummary(ctx context.Context, from, to string) ([]domain.ReportSummary, error) {
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil, &ValidationError{Err: fmt.Errorf("invalid from date: %w", err)}
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil, &ValidationError{Err: fmt.Errorf("invalid to date: %w", err)}
	}
	if fromDate.After(toDate) {
		return nil, &ValidationError{Err: errors.New("from and to must be valid dates with from <= to")}
	}
	return s.reports.Summary(ctx, from, to)
}
