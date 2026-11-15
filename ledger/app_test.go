package ledger_test

import (
	"context"
	"errors"
	"testing"

	"ledger"
)

func TestNewWithCanceledContext(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	svc, closeFn, err := ledger.New(ctx)
	if svc != nil || closeFn != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("New() must return no service and a canceled-context error; got %v", err)
	}
}
