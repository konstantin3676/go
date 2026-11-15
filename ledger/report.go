package ledger

import "context"

func GetReportSummary(ctx context.Context, from, to string) ([]ReportSummary, error) {
	return legacyService().GetReportSummary(ctx, from, to)
}
