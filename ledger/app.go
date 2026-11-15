package ledger

import (
	"context"

	"ledger/internal/app"
)

func New(ctx context.Context) (Service, func() error, error) {
	return app.New(ctx)
}
