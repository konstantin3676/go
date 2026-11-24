package api

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"ledger"
)

const bulkPath = "/api/transactions/bulk"

func TestImportTransactionsHandlerMixed(t *testing.T) {
	want := ledger.ImportSummary{Accepted: 1, Rejected: 1, Errors: []ledger.ImportError{
		{Index: 1, Error: "transaction amount must be positive"},
	}}
	svc := &serviceStub{importTransactions: func(ctx context.Context, txs []ledger.Transaction, workers int) (ledger.ImportSummary, error) {
		wantTxs := []ledger.Transaction{
			{Amount: 18, Category: "food", Description: "lunch", Date: "2025-09-10"},
			{Amount: -1, Category: "books"},
		}
		if ctx != t.Context() || workers != 3 || !reflect.DeepEqual(txs, wantTxs) {
			t.Fatalf("import args = %+v, workers = %d, context forwarded = %v", txs, workers, ctx == t.Context())
		}
		return want, nil
	}}
	response := performJSONRequest(t, NewHandler(svc).Routes(), http.MethodPost, bulkPath+"?workers=3", `[
		{"amount":18,"category":"food","description":"lunch","date":"2025-09-10"},
		{"amount":-1,"category":"books"}
	]`)
	requireImportSummary(t, response.Code, response.Body.Bytes(), want)
	requireJSONContentType(t, response)
}

func TestImportTransactionsHandlerEmptyAndDefaultWorkers(t *testing.T) {
	want := ledger.ImportSummary{Errors: []ledger.ImportError{}}
	svc := &serviceStub{importTransactions: func(_ context.Context, txs []ledger.Transaction, workers int) (ledger.ImportSummary, error) {
		if len(txs) != 0 || workers != 4 {
			t.Fatalf("transactions = %+v, workers = %d; want empty batch, 4 workers", txs, workers)
		}
		return want, nil
	}}
	response := performJSONRequest(t, NewHandler(svc).Routes(), http.MethodPost, bulkPath, `[]`)
	requireImportSummary(t, response.Code, response.Body.Bytes(), want)
	if got := response.Body.String(); got != "{\"accepted\":0,\"rejected\":0,\"errors\":[]}\n" {
		t.Fatalf("unexpected JSON shape: %s", got)
	}
}

func TestImportTransactionsHandlerInvalidInput(t *testing.T) {
	router := NewHandler(&serviceStub{}).Routes()
	for _, body := range []string{``, `{}`, `null`, `[null]`, `[1]`, `[{"amount":"18"}]`, `[`, `[] {}`} {
		t.Run("body="+body, func(t *testing.T) {
			response := performJSONRequest(t, router, http.MethodPost, bulkPath, body)
			requireErrorResponse(t, response, http.StatusBadRequest, "expected a JSON array of transactions")
		})
	}
	for _, query := range []string{"workers=", "workers=-1", "workers=0", "workers=33", "workers=x", "workers=4&workers=5"} {
		t.Run(query, func(t *testing.T) {
			response := performJSONRequest(t, router, http.MethodPost, bulkPath+"?"+query, `[]`)
			requireErrorResponse(t, response, http.StatusBadRequest, "workers must be a single integer from 1 to 32")
		})
	}
}

func TestImportTransactionsHandlerCancellation(t *testing.T) {
	svc := &serviceStub{importTransactions: func(context.Context, []ledger.Transaction, int) (ledger.ImportSummary, error) {
		return ledger.ImportSummary{Accepted: 1}, context.Canceled
	}}
	response := performJSONRequest(t, NewHandler(svc).Routes(), http.MethodPost, bulkPath, `[{"amount":1,"category":"food"}]`)
	requireErrorResponse(t, response, http.StatusGatewayTimeout, timeoutMessage)
}

func TestImportTransactionsHandlerDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc := &serviceStub{importTransactions: func(ctx context.Context, _ []ledger.Transaction, _ int) (ledger.ImportSummary, error) {
			<-ctx.Done()
			return ledger.ImportSummary{Accepted: 1}, ctx.Err()
		}}
		router := TimeoutMiddleware(time.Second, NewHandler(svc).Routes())
		response := performJSONRequest(t, router, http.MethodPost, bulkPath, `[{"amount":1,"category":"food"}]`)
		requireErrorResponse(t, response, http.StatusGatewayTimeout, timeoutMessage)
	})
}

func requireImportSummary(t *testing.T, status int, body []byte, want ledger.ImportSummary) {
	t.Helper()
	var got ledger.ImportSummary
	if err := json.Unmarshal(body, &got); err != nil || status != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Fatalf("import response = %s, status = %d, decode error = %v; want %+v and 200", body, status, err, want)
	}
}
