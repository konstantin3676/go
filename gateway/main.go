package main

import (
	"log"
	"net/http"

	"gateway/internal/api"
)

func main() {
	mux := http.NewServeMux()

	mux.Handle("POST /api/transactions", http.HandlerFunc(api.HandleCreateTransaction))
	mux.Handle("GET /api/transactions", http.HandlerFunc(api.HandleListTransactions))
	mux.Handle("POST /api/budgets", http.HandlerFunc(api.HandleCreateBudget))
	mux.Handle("GET /api/budgets", http.HandlerFunc(api.HandleListBudgets))

	handler := api.LoggingMiddleware(mux)

	log.Println("Gateway service started on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}