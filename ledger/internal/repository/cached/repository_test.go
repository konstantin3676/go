package cached

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"ledger/internal/domain"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type budgetStub struct {
	upsert func(context.Context, domain.Budget) error
	get    func(context.Context, string) (domain.Budget, error)
	list   func(context.Context) ([]domain.Budget, error)
}

func (s budgetStub) Upsert(ctx context.Context, budget domain.Budget) error {
	return s.upsert(ctx, budget)
}

func (s budgetStub) GetByCategory(ctx context.Context, category string) (domain.Budget, error) {
	return s.get(ctx, category)
}

func (s budgetStub) List(ctx context.Context) ([]domain.Budget, error) {
	return s.list(ctx)
}

type reportStub func(context.Context, string, string) ([]domain.ReportSummary, error)

func (s reportStub) Summary(ctx context.Context, from, to string) ([]domain.ReportSummary, error) {
	return s(ctx, from, to)
}

func cacheClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return server, client
}

func TestBudgetListCache(t *testing.T) {
	server, client := cacheClient(t)
	want := []domain.Budget{{Category: "еда", Limit: 500}}
	calls := 0
	ctx := t.Context()
	repo := NewBudgetRepository(budgetStub{list: func(gotCtx context.Context) ([]domain.Budget, error) {
		if gotCtx != ctx {
			t.Fatal("context was not forwarded")
		}
		calls++
		return want, nil
	}}, client)
	for range 2 {
		got, err := repo.List(ctx)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("List() = %+v, %v", got, err)
		}
	}
	if calls != 1 || server.TTL(budgetsKey) != 20*time.Second {
		t.Fatalf("source calls = %d, TTL = %v", calls, server.TTL(budgetsKey))
	}
	server.FastForward(20 * time.Second)
	if _, err := repo.List(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("source calls after expiration = %d, want 2", calls)
	}
}

func TestBudgetUpsertInvalidatesCache(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "success"
		if failed {
			name = "source error"
		}
		t.Run(name, func(t *testing.T) {
			server, client := cacheClient(t)
			server.Set(budgetsKey, "[]")
			want := domain.Budget{Category: "еда", Limit: 500}
			storageErr := errors.New("upsert failed")
			repo := NewBudgetRepository(budgetStub{upsert: func(ctx context.Context, got domain.Budget) error {
				if ctx != t.Context() || got != want {
					t.Fatal("upsert arguments were not forwarded")
				}
				if failed {
					return storageErr
				}
				return nil
			}}, client)
			err := repo.Upsert(t.Context(), want)
			if failed {
				if !errors.Is(err, storageErr) || !server.Exists(budgetsKey) {
					t.Fatalf("failed upsert must preserve cache; error = %v", err)
				}
			} else if err != nil || server.Exists(budgetsKey) {
				t.Fatalf("successful upsert must invalidate cache; error = %v", err)
			}
		})
	}
}

func TestBudgetGetByCategoryBypassesCache(t *testing.T) {
	server, client := cacheClient(t)
	server.Set(budgetsKey, `[{"category":"еда","limit":100}]`)
	want := domain.Budget{Category: "еда", Limit: 500}
	repo := NewBudgetRepository(budgetStub{get: func(ctx context.Context, category string) (domain.Budget, error) {
		if ctx != t.Context() || category != want.Category {
			t.Fatal("get arguments were not forwarded")
		}
		return want, nil
	}}, client)
	got, err := repo.GetByCategory(t.Context(), want.Category)
	if err != nil || got != want {
		t.Fatalf("GetByCategory() = %+v, %v; want current budget", got, err)
	}
}

func TestReportCache(t *testing.T) {
	server, client := cacheClient(t)
	want := []domain.ReportSummary{{Category: "еда", Amount: 250}}
	calls := 0
	repo := NewReportRepository(reportStub(func(ctx context.Context, from, to string) ([]domain.ReportSummary, error) {
		if ctx != t.Context() || from != "2025-09-01" || to != "2025-09-30" {
			t.Fatal("summary arguments were not forwarded")
		}
		calls++
		return want, nil
	}), client)
	for range 2 {
		got, err := repo.Summary(t.Context(), "2025-09-01", "2025-09-30")
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("Summary() = %+v, %v", got, err)
		}
	}
	key := "report:summary:2025-09-01:2025-09-30"
	if calls != 1 || server.TTL(key) != 30*time.Second {
		t.Fatalf("source calls = %d, TTL = %v", calls, server.TTL(key))
	}
	server.FastForward(30 * time.Second)
	if _, err := repo.Summary(t.Context(), "2025-09-01", "2025-09-30"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("source calls after expiration = %d, want 2", calls)
	}
}

func TestRepositoriesWithoutRedis(t *testing.T) {
	want := []domain.Budget{{Category: "еда", Limit: 500}}
	repo := NewBudgetRepository(budgetStub{
		list:   func(context.Context) ([]domain.Budget, error) { return want, nil },
		upsert: func(context.Context, domain.Budget) error { return nil },
	}, nil)
	if got, err := repo.List(t.Context()); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %+v, %v", got, err)
	}
	if err := repo.Upsert(t.Context(), want[0]); err != nil {
		t.Fatal(err)
	}
	wantReport := []domain.ReportSummary{{Category: "еда", Amount: 100}}
	report := NewReportRepository(reportStub(func(context.Context, string, string) ([]domain.ReportSummary, error) {
		return wantReport, nil
	}), nil)
	if got, err := report.Summary(t.Context(), "2025-09-01", "2025-09-30"); err != nil || !reflect.DeepEqual(got, wantReport) {
		t.Fatalf("Summary() = %+v, %v", got, err)
	}
}

func TestCacheFailures(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		server, client := cacheClient(t)
		server.Set(budgetsKey, "not JSON")
		_, err := NewBudgetRepository(budgetStub{}, client).List(t.Context())
		if err == nil || !strings.HasPrefix(err.Error(), "decode budgets cache:") {
			t.Fatalf("error = %v, want decode error", err)
		}
	})
	t.Run("read error", func(t *testing.T) {
		_, client := cacheClient(t)
		_ = client.Close()
		_, err := NewBudgetRepository(budgetStub{}, client).List(t.Context())
		if err == nil || !strings.HasPrefix(err.Error(), "read budgets cache:") {
			t.Fatalf("error = %v, want read error", err)
		}
	})
	t.Run("write error", func(t *testing.T) {
		_, client := cacheClient(t)
		repo := NewBudgetRepository(budgetStub{list: func(context.Context) ([]domain.Budget, error) {
			_ = client.Close()
			return []domain.Budget{}, nil
		}}, client)
		_, err := repo.List(t.Context())
		if err == nil || !strings.HasPrefix(err.Error(), "write budgets cache:") {
			t.Fatalf("error = %v, want write error", err)
		}
	})
	t.Run("invalidation error", func(t *testing.T) {
		_, client := cacheClient(t)
		_ = client.Close()
		repo := NewBudgetRepository(budgetStub{upsert: func(context.Context, domain.Budget) error { return nil }}, client)
		err := repo.Upsert(t.Context(), domain.Budget{})
		if err == nil || !strings.HasPrefix(err.Error(), "invalidate budgets cache:") {
			t.Fatalf("error = %v, want invalidation error", err)
		}
	})
	t.Run("source error", func(t *testing.T) {
		server, client := cacheClient(t)
		want := errors.New("source failed")
		repo := NewBudgetRepository(budgetStub{list: func(context.Context) ([]domain.Budget, error) { return nil, want }}, client)
		if _, err := repo.List(t.Context()); !errors.Is(err, want) {
			t.Fatalf("error = %v, want source error", err)
		}
		if server.Exists(budgetsKey) {
			t.Fatal("failed result must not be cached")
		}
	})
}
