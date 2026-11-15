package ledger

import (
	"context"
	"fmt"
	"time"

	"ledger/internal/repository/cached"
	"ledger/internal/repository/pg"
)

func GetReportSummary(ctx context.Context, from, to string) ([]ReportSummary, error) {
	if _, err := time.Parse("2006-01-02", from); err != nil {
		return nil, fmt.Errorf("invalid from date: %w", err)
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		return nil, fmt.Errorf("invalid to date: %w", err)
	}

	repo := cached.NewReportRepository(pg.NewReportRepository(DB), redisClient)
	return repo.Summary(ctx, from, to)
}
