package service_test

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"ledger/internal/domain"
	"ledger/internal/service"
)

type reportDataStub struct {
	categories func(context.Context, string, string) ([]string, error)
	sum        func(context.Context, string, string, string) (float64, error)
}

func (s reportDataStub) Categories(ctx context.Context, from, to string) ([]string, error) {
	return s.categories(ctx, from, to)
}

func (s reportDataStub) SumByCategory(ctx context.Context, category, from, to string) (float64, error) {
	return s.sum(ctx, category, from, to)
}

func TestReportCalculatorRunsCategoriesConcurrently(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	started := make(chan struct{}, 3)
	allStarted := make(chan struct{})
	go func() {
		for range 3 {
			select {
			case <-started:
			case <-ctx.Done():
				return
			}
		}
		close(allStarted)
	}()
	source := reportDataStub{
		categories: func(got context.Context, from, to string) ([]string, error) {
			if got.Done() == nil || from != "2025-09-01" || to != "2025-09-30" {
				t.Error("category arguments not forwarded")
			}
			return []string{"еда", "транспорт", "книги"}, nil
		},
		sum: func(got context.Context, category, from, to string) (float64, error) {
			if from != "2025-09-01" || to != "2025-09-30" {
				t.Error("sum period not forwarded")
			}
			started <- struct{}{}
			// A sequential implementation cannot pass this barrier.
			select {
			case <-allStarted:
				return map[string]float64{"еда": 250.5, "транспорт": 100, "книги": 75}[category], nil
			case <-got.Done():
				return 0, got.Err()
			}
		},
	}
	got, err := service.NewReportCalculator(source).Summary(ctx, "2025-09-01", "2025-09-30")
	want := []domain.ReportSummary{{Category: "еда", Amount: 250.5}, {Category: "транспорт", Amount: 100}, {Category: "книги", Amount: 75}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Summary() = %v, %v; want %v", got, err, want)
	}
}

func TestReportCalculatorCancellationWaitsForWorkers(t *testing.T) {
	for _, failWorker := range []bool{false, true} {
		t.Run(map[bool]string{false: "parent canceled", true: "worker failed"}[failWorker], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			started := make(chan struct{}, 2)
			release := make(chan struct{})
			var active atomic.Int32
			storageErr := errors.New("storage failed")
			source := reportDataStub{
				categories: func(context.Context, string, string) ([]string, error) { return []string{"a", "b"}, nil },
				sum: func(got context.Context, category, _, _ string) (float64, error) {
					active.Add(1)
					defer active.Add(-1)
					started <- struct{}{}
					if failWorker && category == "a" {
						select {
						case <-release:
							return 0, storageErr
						case <-got.Done():
							return 0, got.Err()
						}
					}
					<-got.Done()
					return 0, got.Err()
				},
			}
			go func() {
				for range 2 {
					select {
					case <-started:
					case <-ctx.Done():
						return
					}
				}
				if failWorker {
					close(release)
				} else {
					cancel()
				}
			}()
			got, err := service.NewReportCalculator(source).Summary(ctx, "2025-09-01", "2025-09-30")
			wantErr := context.Canceled
			if failWorker {
				wantErr = storageErr
			}
			if got != nil || !errors.Is(err, wantErr) || active.Load() != 0 {
				t.Fatalf("Summary() = %v, %v; active = %d", got, err, active.Load())
			}
		})
	}
}

func TestReportCalculatorEmptyAndCategoryErrors(t *testing.T) {
	storageErr := errors.New("categories failed")
	for _, wantErr := range []error{nil, storageErr} {
		source := reportDataStub{categories: func(context.Context, string, string) ([]string, error) {
			return nil, wantErr
		}}
		got, err := service.NewReportCalculator(source).Summary(t.Context(), "2025-09-01", "2025-09-30")
		if !errors.Is(err, wantErr) || (wantErr == nil && got == nil) || len(got) != 0 {
			t.Fatalf("Summary() = %v, %v", got, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.NewReportCalculator(reportDataStub{}).Summary(ctx, "2025-09-01", "2025-09-30"); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled report: %v", err)
	}
}
