package ledger

import (
	"context"

	"ledger/internal/repository/cached"
	"ledger/internal/repository/memory"
	"ledger/internal/repository/pg"
	"ledger/internal/service"
)

// Keep the package-level API working until Gateway switches to an injected
// service. New application instances will be assembled by the Ledger factory.
var legacyStore = memory.NewStore()

var legacyMemoryService = service.New(
	memory.NewBudgetRepository(legacyStore),
	memory.NewTransactionRepository(legacyStore),
	legacyReportRepository{},
)

func legacyService() Service {
	if DB == nil {
		return legacyMemoryService
	}
	return service.New(
		cached.NewBudgetRepository(pg.NewBudgetRepository(DB), redisClient),
		pg.NewTransactionRepository(DB),
		cached.NewReportRepository(pg.NewReportRepository(DB), redisClient),
	)
}

// Resolve the legacy Redis client at call time, including when DB is nil.
type legacyReportRepository struct{}

func (legacyReportRepository) Summary(ctx context.Context, from, to string) ([]ReportSummary, error) {
	return cached.NewReportRepository(pg.NewReportRepository(DB), redisClient).Summary(ctx, from, to)
}
