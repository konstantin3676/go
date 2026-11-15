package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunStopsOnInitializationError(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := run(ctx)
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "initialize Ledger:") {
		t.Fatalf("run() error = %v, want Ledger initialization error", err)
	}
}
