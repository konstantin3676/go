package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutMiddleware(t *testing.T) {
	t.Run("success and deadline", func(t *testing.T) {
		var ctx context.Context
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx = r.Context()
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > DefaultRequestTimeout {
				t.Fatal("missing request deadline")
			}
			writeJSON(w, http.StatusCreated, map[string]string{"result": "ok"})
		})
		response := performJSONRequest(t, TimeoutMiddleware(DefaultRequestTimeout, next), "GET", "/", "")
		if response.Code != http.StatusCreated || response.Body.String() != "{\"result\":\"ok\"}\n" {
			t.Fatalf("response = %d %s", response.Code, response.Body.String())
		}
		requireJSONContentType(t, response)
		if ctx.Err() != context.Canceled {
			t.Fatal("context not released after successful request")
		}
	})
	t.Run("expiry discards buffered success and waits for cleanup", func(t *testing.T) {
		finished := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"result": "partial"})
			<-r.Context().Done()
			finished = true
		})
		response := performJSONRequest(t, TimeoutMiddleware(time.Millisecond, next), "GET", "/", "")
		requireErrorResponse(t, response, http.StatusGatewayTimeout, timeoutMessage)
		if !finished {
			t.Fatal("handler cleanup not completed")
		}
	})
	t.Run("parent cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("canceled handler called") })
		response := httptest.NewRecorder()
		TimeoutMiddleware(DefaultRequestTimeout, next).ServeHTTP(response, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
		requireErrorResponse(t, response, http.StatusGatewayTimeout, timeoutMessage)
	})
}

func TestTimeoutAppliesToAllAPIRoutes(t *testing.T) {
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/api/transactions", ""},
		{"POST", "/api/transactions", `{}`},
		{"GET", "/api/budgets", ""},
		{"POST", "/api/budgets", `{}`},
		{"GET", "/api/reports/summary?from=2025-09-01&to=2025-09-30", ""},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			handler := TimeoutMiddleware(-time.Second, NewHandler(&serviceStub{}).Routes())
			response := performJSONRequest(t, handler, route.method, route.path, route.body)
			requireErrorResponse(t, response, http.StatusGatewayTimeout, timeoutMessage)
		})
	}
}

func TestContextServiceErrors(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		response := httptest.NewRecorder()
		writeServiceError(response, fmt.Errorf("repository: %w", err))
		requireErrorResponse(t, response, http.StatusGatewayTimeout, timeoutMessage)
	}
}
