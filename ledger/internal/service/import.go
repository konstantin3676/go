package service

import (
	"context"
	"errors"
	"sort"
	"sync"

	"ledger/internal/domain"
)

type ImportSummary struct {
	Accepted int           `json:"accepted"`
	Rejected int           `json:"rejected"`
	Errors   []ImportError `json:"errors"`
}

type ImportError struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

type importJob struct {
	index int
	tx    domain.Transaction
}

type importResult struct {
	index int
	err   error
}

// ImportTransactions returns a partial summary and the context error if interrupted.
func (s *service) ImportTransactions(ctx context.Context, transactions []domain.Transaction, workers int) (ImportSummary, error) {
	summary := ImportSummary{Errors: []ImportError{}}
	if workers <= 0 {
		return summary, &ValidationError{Err: errors.New("workers must be positive")}
	}
	if err := ctx.Err(); err != nil || len(transactions) == 0 {
		return summary, err
	}

	jobs := make(chan importJob)
	results := make(chan importResult)
	collected := make(chan struct{})
	var countsMu sync.Mutex
	go func() {
		defer close(collected)
		for result := range results {
			countsMu.Lock()
			if result.err == nil {
				summary.Accepted++
			} else {
				summary.Rejected++
			}
			countsMu.Unlock()
			if result.err != nil {
				summary.Errors = append(summary.Errors, ImportError{Index: result.index, Error: result.err.Error()})
			}
		}
		sort.Slice(summary.Errors, func(i, j int) bool { return summary.Errors[i].Index < summary.Errors[j].Index })
	}()

	var pool sync.WaitGroup
	for range min(workers, len(transactions)) {
		pool.Add(1)
		go func() {
			defer pool.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok || ctx.Err() != nil {
						return
					}
					_, err := s.AddTransaction(ctx, job.tx)
					if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
						return // an interrupted operation is not a business rejection
					}
					// The collector drains until closure, even on cancellation. Do not drop a completed result.
					results <- importResult{index: job.index, err: err}
				}
			}
		}()
	}

sendJobs:
	for index, tx := range transactions {
		select {
		case <-ctx.Done():
			break sendJobs
		case jobs <- importJob{index: index, tx: tx}:
		}
	}
	close(jobs)
	pool.Wait()
	close(results)
	<-collected
	return summary, ctx.Err()
}
