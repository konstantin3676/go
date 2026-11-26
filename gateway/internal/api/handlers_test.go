package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"ledger"
)

type serviceStub struct {
	setBudget          func(context.Context, ledger.Budget) error
	listBudgets        func(context.Context) ([]ledger.Budget, error)
	addTransaction     func(context.Context, ledger.Transaction) (ledger.Transaction, error)
	importTransactions func(context.Context, []ledger.Transaction, int) (ledger.ImportSummary, error)
	listTransactions   func(context.Context) ([]ledger.Transaction, error)
	report             func(context.Context, string, string) ([]ledger.ReportSummary, error)
}

var _ ledger.Service = (*serviceStub)(nil)

func (s *serviceStub) SetBudget(ctx context.Context, budget ledger.Budget) error {
	return s.setBudget(ctx, budget)
}

func (s *serviceStub) ListBudgets(ctx context.Context) ([]ledger.Budget, error) {
	return s.listBudgets(ctx)
}

func (s *serviceStub) AddTransaction(ctx context.Context, tx ledger.Transaction) (ledger.Transaction, error) {
	return s.addTransaction(ctx, tx)
}

func (s *serviceStub) ImportTransactions(ctx context.Context, txs []ledger.Transaction, workers int) (ledger.ImportSummary, error) {
	return s.importTransactions(ctx, txs, workers)
}

func (s *serviceStub) ListTransactions(ctx context.Context) ([]ledger.Transaction, error) {
	return s.listTransactions(ctx)
}

func (s *serviceStub) GetReportSummary(ctx context.Context, from, to string) ([]ledger.ReportSummary, error) {
	return s.report(ctx, from, to)
}

func (s *serviceStub) LoadBudgets(context.Context, io.Reader) error {
	panic("LoadBudgets must not be called by HTTP handlers")
}

func performJSONRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(t.Context())
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
	if len(body) != 1 || body["error"] != message {
		t.Fatalf("error body = %v, want error %q", body, message)
	}
}

func TestPing(t *testing.T) {
	router := NewHandler(nil).Routes()
	response := performJSONRequest(t, router, http.MethodGet, "/ping", "")
	if response.Code != http.StatusOK || response.Body.String() != "pong" {
		t.Fatalf("GET /ping: status = %d, body = %q; want 200 and pong", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
}

func TestBudgetHandlers(t *testing.T) {
	var saved ledger.Budget
	setCalls, listCalls := 0, 0
	svc := &serviceStub{
		setBudget: func(ctx context.Context, budget ledger.Budget) error {
			setCalls++
			if ctx != t.Context() || budget != (ledger.Budget{Category: "еда", Limit: 5000}) {
				t.Fatalf("SetBudget arguments = %+v; context forwarded = %v", budget, ctx == t.Context())
			}
			saved = budget
			return nil
		},
		listBudgets: func(ctx context.Context) ([]ledger.Budget, error) {
			listCalls++
			if ctx != t.Context() {
				t.Fatal("request context was not forwarded")
			}
			return []ledger.Budget{saved}, nil
		},
	}
	router := NewHandler(svc).Routes()
	created := performJSONRequest(t, router, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body = %s", created.Code, created.Body.String())
	}
	requireJSONContentType(t, created)
	var budget BudgetResponse
	if err := json.NewDecoder(created.Body).Decode(&budget); err != nil || budget != (BudgetResponse{Category: "еда", Limit: 5000}) {
		t.Fatalf("created budget = %+v, %v", budget, err)
	}
	listed := performJSONRequest(t, router, http.MethodGet, "/api/budgets", "")
	requireJSONContentType(t, listed)
	var budgets []BudgetResponse
	if err := json.NewDecoder(listed.Body).Decode(&budgets); err != nil || listed.Code != http.StatusOK || !reflect.DeepEqual(budgets, []BudgetResponse{budget}) {
		t.Fatalf("budget list = %+v, %v; status = %d", budgets, err, listed.Code)
	}
	if setCalls != 1 || listCalls != 1 {
		t.Fatalf("service calls: set = %d, list = %d", setCalls, listCalls)
	}
}

func TestTransactionHandlers(t *testing.T) {
	want := ledger.Transaction{Amount: 450, Category: "еда", Description: "ланч", Date: "2025-09-10"}
	var saved ledger.Transaction
	addCalls, listCalls := 0, 0
	svc := &serviceStub{
		addTransaction: func(ctx context.Context, tx ledger.Transaction) (ledger.Transaction, error) {
			addCalls++
			if ctx != t.Context() || tx != want {
				t.Fatalf("AddTransaction arguments = %+v; context forwarded = %v", tx, ctx == t.Context())
			}
			tx.ID = 42
			saved = tx
			return tx, nil
		},
		listTransactions: func(ctx context.Context) ([]ledger.Transaction, error) {
			listCalls++
			if ctx != t.Context() {
				t.Fatal("request context was not forwarded")
			}
			return []ledger.Transaction{saved}, nil
		},
	}
	router := NewHandler(svc).Routes()
	created := performJSONRequest(t, router, http.MethodPost, "/api/transactions", `{"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body = %s", created.Code, created.Body.String())
	}
	requireJSONContentType(t, created)
	var tx TransactionResponse
	if err := json.NewDecoder(created.Body).Decode(&tx); err != nil || tx != NewTransactionResponse(saved) {
		t.Fatalf("created transaction = %+v, %v", tx, err)
	}
	listed := performJSONRequest(t, router, http.MethodGet, "/api/transactions", "")
	requireJSONContentType(t, listed)
	var transactions []TransactionResponse
	if err := json.NewDecoder(listed.Body).Decode(&transactions); err != nil || listed.Code != http.StatusOK || !reflect.DeepEqual(transactions, []TransactionResponse{tx}) {
		t.Fatalf("transaction list = %+v, %v; status = %d", transactions, err, listed.Code)
	}
	if addCalls != 1 || listCalls != 1 {
		t.Fatalf("service calls: add = %d, list = %d", addCalls, listCalls)
	}
}

func TestMalformedRequestsDoNotCallService(t *testing.T) {
	for _, path := range []string{"/api/budgets", "/api/transactions"} {
		t.Run(path, func(t *testing.T) {
			response := performJSONRequest(t, NewHandler(&serviceStub{}).Routes(), http.MethodPost, path, `{"category":`)
			requireErrorResponse(t, response, http.StatusBadRequest, "invalid request body")
		})
	}
}

func TestServiceErrorResponses(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body string
		serviceErr               error
		status                   int
		message                  string
	}{
		{"invalid budget", "POST", "/api/budgets", `{"category":"еда","limit":0}`, &ledger.ValidationError{Err: errors.New("budget limit must be positive")}, 400, "budget limit must be positive"},
		{"invalid transaction", "POST", "/api/transactions", `{"amount":-100}`, &ledger.ValidationError{Err: errors.New("transaction amount must be positive")}, 400, "transaction amount must be positive"},
		{"budget exceeded", "POST", "/api/transactions", `{"amount":600,"category":"еда"}`, fmt.Errorf("%w for category %q", ledger.ErrBudgetExceeded, "еда"), 409, "budget exceeded"},
		{"budget write failed", "POST", "/api/budgets", `{"category":"еда","limit":500}`, errors.New("upsert budget: unavailable"), 500, "upsert budget: unavailable"},
		{"transaction write failed", "POST", "/api/transactions", `{"amount":100,"category":"еда"}`, errors.New("insert transaction: unavailable"), 500, "insert transaction: unavailable"},
		{"budget list failed", "GET", "/api/budgets", "", errors.New("list budgets: unavailable"), 500, "list budgets: unavailable"},
		{"transaction list failed", "GET", "/api/transactions", "", errors.New("list transactions: unavailable"), 500, "list transactions: unavailable"},
		{"report validation error", "GET", "/api/reports/summary?from=2025-09-01&to=2025-09-30", "", &ledger.ValidationError{Err: errors.New("invalid period")}, 400, "from and to must be valid dates with from <= to"},
		{"report failed", "GET", "/api/reports/summary?from=2025-09-01&to=2025-09-30", "", errors.New("query report summary: unavailable"), 500, "query report summary: unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			called := func(ctx context.Context) {
				calls++
				if ctx != t.Context() {
					t.Fatal("request context was not forwarded")
				}
			}
			svc := &serviceStub{
				setBudget:   func(ctx context.Context, _ ledger.Budget) error { called(ctx); return tc.serviceErr },
				listBudgets: func(ctx context.Context) ([]ledger.Budget, error) { called(ctx); return nil, tc.serviceErr },
				addTransaction: func(ctx context.Context, _ ledger.Transaction) (ledger.Transaction, error) {
					called(ctx)
					return ledger.Transaction{}, tc.serviceErr
				},
				listTransactions: func(ctx context.Context) ([]ledger.Transaction, error) { called(ctx); return nil, tc.serviceErr },
				report: func(ctx context.Context, _, _ string) ([]ledger.ReportSummary, error) {
					called(ctx)
					return nil, tc.serviceErr
				},
			}
			response := performJSONRequest(t, NewHandler(svc).Routes(), tc.method, tc.path, tc.body)
			requireErrorResponse(t, response, tc.status, tc.message)
			if calls != 1 {
				t.Fatalf("service calls = %d, want 1", calls)
			}
		})
	}
}

func TestEmptyLists(t *testing.T) {
	svc := &serviceStub{
		listBudgets:      func(context.Context) ([]ledger.Budget, error) { return nil, nil },
		listTransactions: func(context.Context) ([]ledger.Transaction, error) { return nil, nil },
	}
	for _, path := range []string{"/api/budgets", "/api/transactions"} {
		response := performJSONRequest(t, NewHandler(svc).Routes(), http.MethodGet, path, "")
		if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != "[]" {
			t.Fatalf("GET %s: status = %d; body = %s", path, response.Code, response.Body.String())
		}
		requireJSONContentType(t, response)
	}
}

func TestReportSummary(t *testing.T) {
	svc := &serviceStub{report: func(ctx context.Context, from, to string) ([]ledger.ReportSummary, error) {
		if ctx != t.Context() || from != "2025-09-01" || to != "2025-09-30" {
			t.Fatalf("report arguments = %q, %q; context forwarded = %v", from, to, ctx == t.Context())
		}
		return []ledger.ReportSummary{{Category: "еда", Amount: 450.5}, {Category: "транспорт", Amount: 100}}, nil
	}}
	response := performJSONRequest(t, NewHandler(svc).Routes(), http.MethodGet, "/api/reports/summary?from=2025-09-01&to=2025-09-30", "")
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"еда":450.5,"транспорт":100}` {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	requireJSONContentType(t, response)
}

func TestReportInvalidQueryParametersDoNotCallService(t *testing.T) {
	for _, query := range []string{
		"?from=bad&to=2025-09-30",
		"?from=2025-09-01&to=bad",
		"?from=2025-02-29&to=2025-09-30",
		"?from=2025-09-30&to=2025-09-01",
		"?from=2025-9-01&to=2025-09-30",
	} {
		t.Run(query, func(t *testing.T) {
			response := performJSONRequest(t, NewHandler(&serviceStub{}).Routes(), "GET", "/api/reports/summary"+query, "")
			requireErrorResponse(t, response, http.StatusBadRequest, "from and to must be valid dates with from <= to")
		})
	}
}

func TestReportEmptyAndSingleDay(t *testing.T) {
	svc := &serviceStub{report: func(_ context.Context, from, to string) ([]ledger.ReportSummary, error) {
		if from != to {
			t.Fatal("single day range not forwarded")
		}
		return nil, nil
	}}
	response := performJSONRequest(t, NewHandler(svc).Routes(), "GET", "/api/reports/summary?from=2025-09-10&to=2025-09-10", "")
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{}` {
		t.Fatalf("empty report = %d %s", response.Code, response.Body.String())
	}
	requireJSONContentType(t, response)
}

func TestReportDeadline(t *testing.T) {
	finished := false
	svc := &serviceStub{report: func(ctx context.Context, _, _ string) ([]ledger.ReportSummary, error) {
		<-ctx.Done()
		finished = true
		return nil, fmt.Errorf("report interrupted: %w", ctx.Err())
	}}
	handler := TimeoutMiddleware(10*time.Millisecond, NewHandler(svc).Routes())
	response := performJSONRequest(t, handler, "GET", "/api/reports/summary?from=2025-09-01&to=2025-09-30", "")
	requireErrorResponse(t, response, http.StatusGatewayTimeout, timeoutMessage)
	if !finished {
		t.Fatal("report still running after response")
	}
}

func TestReportWrappedCancellation(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		svc := &serviceStub{report: func(context.Context, string, string) ([]ledger.ReportSummary, error) {
			return nil, fmt.Errorf("report: %w", err)
		}}
		response := performJSONRequest(t, NewHandler(svc).Routes(), "GET", "/api/reports/summary?from=2025-09-01&to=2025-09-30", "")
		requireErrorResponse(t, response, http.StatusGatewayTimeout, timeoutMessage)
	}
}

func TestReportMissingQueryParameters(t *testing.T) {
	for _, query := range []string{"", "?from=2025-09-01", "?to=2025-09-30"} {
		response := performJSONRequest(t, NewHandler(&serviceStub{}).Routes(), http.MethodGet, "/api/reports/summary"+query, "")
		requireErrorResponse(t, response, http.StatusBadRequest, "from and to query parameters are required")
	}
}
