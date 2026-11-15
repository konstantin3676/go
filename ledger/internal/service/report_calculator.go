package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"ledger/internal/domain"
)

type reportCalculator struct {
	source domain.ReportDataRepository
}

var _ domain.ReportRepository = (*reportCalculator)(nil)

func NewReportCalculator(source domain.ReportDataRepository) domain.ReportRepository {
	return &reportCalculator{source: source}
}

func (c *reportCalculator) Summary(ctx context.Context, from, to string) ([]domain.ReportSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	workCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	stopHeartbeat := startReportHeartbeat(workCtx, 400*time.Millisecond, log.Default())
	defer stopHeartbeat()
	categories, err := c.source.Categories(workCtx, from, to)
	if err != nil {
		cancel(err)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}

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

func startReportHeartbeat(ctx context.Context, interval time.Duration, logger *log.Logger) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer close(stopped)
		defer func() {
			ticker.Stop()
			if err := context.Cause(ctx); err != nil {
				logger.Printf("report heartbeat stopped; calculation interrupted (ctx.Done): %v", err)
			} else {
				logger.Print("report heartbeat stopped; calculation finished")
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				logger.Print("report calculation in progress")
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}
