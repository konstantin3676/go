package main

import (
	"context"
	"log"
	"net/http"

	"gateway/internal/api"
	"ledger"
)

func main() {
	if _, err := ledger.InitDB(context.Background()); err != nil {
		log.Fatalf("could not connect to PostgreSQL: %v", err)
	}
	defer ledger.CloseDB()

	if _, err := ledger.InitRedis(context.Background()); err != nil {
		log.Fatalf("could not connect to Redis: %v", err)
	}
	defer ledger.CloseRedis()

	mux := http.NewServeMux()

	mux.Handle("POST /api/transactions", http.HandlerFunc(api.HandleCreateTransaction))
	mux.Handle("GET /api/transactions", http.HandlerFunc(api.HandleListTransactions))
	mux.Handle("POST /api/budgets", http.HandlerFunc(api.HandleCreateBudget))
	mux.Handle("GET /api/budgets", http.HandlerFunc(api.HandleListBudgets))
	mux.Handle("GET /api/reports/summary", http.HandlerFunc(api.HandleReportSummary))

	handler := api.LoggingMiddleware(mux)

	log.Println("Gateway service started on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
