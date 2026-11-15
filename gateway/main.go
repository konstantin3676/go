package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"gateway/internal/api"
	"ledger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	svc, closeFn, err := ledger.New(startupCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("initialize Ledger: %w", err)
	}
	defer closeFn()

	server := &http.Server{
		Addr:    ":8080",
		Handler: api.LoggingMiddleware(api.NewHandler(svc).Routes()),
	}
	serverErr := make(chan error, 1)
	log.Println("Gateway service started on :8080")
	go func() { serverErr <- server.ListenAndServe() }()
	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("shutdown HTTP: %w", err)
		}
		return nil
	}
}
