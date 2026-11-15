package cached

import (
	"context"
	"fmt"
	"time"

	"ledger/internal/domain"

	"github.com/redis/go-redis/v9"
)

type ReportRepository struct {
	source domain.ReportRepository
	client *redis.Client
}

var _ domain.ReportRepository = (*ReportRepository)(nil)

func NewReportRepository(source domain.ReportRepository, client *redis.Client) *ReportRepository {
	return &ReportRepository{source: source, client: client}
}

func (r *ReportRepository) Summary(ctx context.Context, from, to string) ([]domain.ReportSummary, error) {
	key := fmt.Sprintf("report:summary:%s:%s", from, to)
	if result, found, err := readJSON[[]domain.ReportSummary](ctx, r.client, key, "report"); err != nil {
		return nil, err
	} else if found {
		return result, nil
	}
	result, err := r.source.Summary(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if err := writeJSON(ctx, r.client, key, "report", result, 30*time.Second); err != nil {
		return nil, err
	}
	return result, nil
}
