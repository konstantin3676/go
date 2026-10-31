package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"ledger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if _, err := ledger.InitDB(ctx); err != nil {
		log.Fatalf("could not connect to PostgreSQL: %v", err)
	}
	defer ledger.CloseDB()
	log.Println("Ledger connected to PostgreSQL")

	if _, err := ledger.InitRedis(ctx); err != nil {
		log.Fatalf("could not connect to Redis: %v", err)
	}
	defer ledger.CloseRedis()
	log.Println("Ledger connected to Redis")

	log.Println("Ledger service started")
	<-ctx.Done()
	log.Println("Ledger service stopped")
}
