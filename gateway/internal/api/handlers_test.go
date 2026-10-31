package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ledger"
)

func testRouter() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /api/transactions", http.HandlerFunc(HandleCreateTransaction))
	mux.Handle("GET /api/transactions", http.HandlerFunc(HandleListTransactions))
	mux.Handle("POST /api/budgets", http.HandlerFunc(HandleCreateBudget))
	mux.Handle("GET /api/budgets", http.HandlerFunc(HandleListBudgets))
	mux.Handle("GET /api/reports/summary", http.HandlerFunc(HandleReportSummary))
	return mux
}

func performJSONRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func requireJSONContentType(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	want := "application/json; charset=utf-8"
	if got := response.Header().Get("Content-Type"); got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
}

func requireErrorResponse(t *testing.T, response *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, status, response.Body.String())
	}
	requireJSONContentType(t, response)

	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body["error"] != message {
		t.Fatalf("error = %q, want %q", body["error"], message)
	}
}

func TestBudgetHandlers(t *testing.T) {
	router := testRouter()

	t.Run("create and list", func(t *testing.T) {
		ledger.Reset()
		t.Cleanup(ledger.Reset)

		create := performJSONRequest(t, router, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`)
		if create.Code != http.StatusCreated {
			t.Fatalf("POST /api/budgets status = %d, want %d", create.Code, http.StatusCreated)
		}
		requireJSONContentType(t, create)

		var created BudgetResponse
		if err := json.NewDecoder(create.Body).Decode(&created); err != nil {
			t.Fatalf("decode created budget: %v", err)
		}
		if created.Category != "еда" || created.Limit != 5000 {
			t.Fatalf("created budget = %+v, want category еда and limit 5000", created)
		}

		list := performJSONRequest(t, router, http.MethodGet, "/api/budgets", "")
		if list.Code != http.StatusOK {
			t.Fatalf("GET /api/budgets status = %d, want %d", list.Code, http.StatusOK)
		}
		requireJSONContentType(t, list)

		var budgets []BudgetResponse
		if err := json.NewDecoder(list.Body).Decode(&budgets); err != nil {
			t.Fatalf("decode budget list: %v", err)
		}
		if len(budgets) != 1 || budgets[0].Category != "еда" || budgets[0].Limit != 5000 {
			t.Fatalf("budget list = %+v, want one budget for еда with limit 5000", budgets)
		}
	})

	t.Run("bad_json", func(t *testing.T) {
		ledger.Reset()
		t.Cleanup(ledger.Reset)

		response := performJSONRequest(t, router, http.MethodPost, "/api/budgets", `{"category":`)
		requireErrorResponse(t, response, http.StatusBadRequest, "invalid request body")
	})

	t.Run("non-positive limit", func(t *testing.T) {
		ledger.Reset()
		t.Cleanup(ledger.Reset)

		response := performJSONRequest(t, router, http.MethodPost, "/api/budgets", `{"category":"еда","limit":0}`)
		requireErrorResponse(t, response, http.StatusBadRequest, "budget limit must be positive")
	})
}

func TestTransactionHandlers(t *testing.T) {
	router := testRouter()

	t.Run("ok", func(t *testing.T) {
		ledger.Reset()
		t.Cleanup(ledger.Reset)

		budget := performJSONRequest(t, router, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`)
		if budget.Code != http.StatusCreated {
			t.Fatalf("creating budget status = %d, want %d", budget.Code, http.StatusCreated)
		}

		transaction := performJSONRequest(t, router, http.MethodPost, "/api/transactions", `{"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}`)
		if transaction.Code != http.StatusCreated {
			t.Fatalf("POST /api/transactions status = %d, want %d; body = %s", transaction.Code, http.StatusCreated, transaction.Body.String())
		}
		requireJSONContentType(t, transaction)

		var created TransactionResponse
		if err := json.NewDecoder(transaction.Body).Decode(&created); err != nil {
			t.Fatalf("decode created transaction: %v", err)
		}
		want := TransactionResponse{ID: 1, Amount: 450, Category: "еда", Description: "ланч", Date: "2025-09-10"}
		if created != want {
			t.Fatalf("created transaction = %+v, want %+v", created, want)
		}

		list := performJSONRequest(t, router, http.MethodGet, "/api/transactions", "")
		if list.Code != http.StatusOK {
			t.Fatalf("GET /api/transactions status = %d, want %d", list.Code, http.StatusOK)
		}
		requireJSONContentType(t, list)

		var transactions []TransactionResponse
		if err := json.NewDecoder(list.Body).Decode(&transactions); err != nil {
			t.Fatalf("decode transaction list: %v", err)
		}
		if len(transactions) != 1 || transactions[0] != want {
			t.Fatalf("transaction list = %+v, want [%+v]", transactions, want)
		}
	})

	t.Run("exceeded", func(t *testing.T) {
		ledger.Reset()
		t.Cleanup(ledger.Reset)

		budget := performJSONRequest(t, router, http.MethodPost, "/api/budgets", `{"category":"еда","limit":500}`)
		if budget.Code != http.StatusCreated {
			t.Fatalf("creating budget status = %d, want %d", budget.Code, http.StatusCreated)
		}
		withinLimit := performJSONRequest(t, router, http.MethodPost, "/api/transactions", `{"amount":400,"category":"еда","date":"2025-09-10"}`)
		if withinLimit.Code != http.StatusCreated {
			t.Fatalf("transaction within budget status = %d, want %d", withinLimit.Code, http.StatusCreated)
		}

		exceeded := performJSONRequest(t, router, http.MethodPost, "/api/transactions", `{"amount":200,"category":"еда","date":"2025-09-11"}`)
		requireErrorResponse(t, exceeded, http.StatusConflict, "budget exceeded")
		transactions, err := ledger.ListTransactions()
		if err != nil {
			t.Fatalf("ListTransactions() returned an error: %v", err)
		}
		if got := len(transactions); got != 1 {
			t.Fatalf("transaction count after rejected request = %d, want 1", got)
		}
	})

	t.Run("bad_json", func(t *testing.T) {
		ledger.Reset()
		t.Cleanup(ledger.Reset)

		response := performJSONRequest(t, router, http.MethodPost, "/api/transactions", `{"amount":`)
		requireErrorResponse(t, response, http.StatusBadRequest, "invalid request body")
	})
}
