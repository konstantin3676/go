package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gateway/internal/api"
)

func TestRunStopsOnInitializationError(t *testing.T) {
	t.Setenv("REQUEST_TIMEOUT", "")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := run(ctx)
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "initialize Ledger:") {
		t.Fatalf("run() error = %v, want Ledger initialization error", err)
	}
}

func TestRequestTimeout(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    time.Duration
		invalid bool
	}{
		{"", api.DefaultRequestTimeout, false},
		{"50ms", 50 * time.Millisecond, false},
		{"bad", 0, true},
		{"0s", 0, true},
		{"-1s", 0, true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("REQUEST_TIMEOUT", tc.value)
			got, err := requestTimeout()
			if got != tc.want || (err != nil) != tc.invalid {
				t.Fatalf("requestTimeout() = %v, %v", got, err)
			}
		})
	}
}
