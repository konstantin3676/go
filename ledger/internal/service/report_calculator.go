package service

import (
	"context"
	"fmt"
	"sync"

	"ledger/internal/domain"
)

type reportCalculator struct {
	source domain.ReportDataRepository
}

var _ domain.ReportRepository = (*reportCalculator)(nil)

// NewReportCalculator builds the concurrent computation behind the report cache.
func NewReportCalculator(source domain.ReportDataRepository) domain.ReportRepository {
	return &reportCalculator{source: source}
}

func (c *reportCalculator) Summary(ctx context.Context, from, to string) ([]domain.ReportSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	workCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	categories, err := c.source.Categories(workCtx, from, to)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}

	// Each worker owns one slice element. Only read results after Wait.
	results := make([]domain.ReportSummary, len(categories))
	var wg sync.WaitGroup
	for i, category := range categories {
		if workCtx.Err() != nil {
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if workCtx.Err() != nil {
				return
			}
			amount, err := c.source.SumByCategory(workCtx, category, from, to)
			if err != nil {
				cancel(fmt.Errorf("calculate category %q: %w", category, err))
				return
			}
			results[i] = domain.ReportSummary{Category: category, Amount: amount}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := context.Cause(workCtx); err != nil {
		return nil, err
	}
	return results, nil
}
