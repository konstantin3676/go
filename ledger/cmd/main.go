package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

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
	_, closeFn, err := ledger.New(startupCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("initialize Ledger: %w", err)
	}
	defer closeFn()
	log.Println("Ledger connected to PostgreSQL")
	log.Println("Ledger connected to Redis")

	log.Println("Ledger service started")
	<-ctx.Done()
	log.Println("Ledger service stopped")
	return nil
}
