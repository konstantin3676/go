package api

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"time"
)

const DefaultRequestTimeout = 2 * time.Second

const timeoutMessage = "request timed out or canceled"

func TimeoutMiddleware(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		if ctx.Err() != nil {
			writeError(w, http.StatusGatewayTimeout, timeoutMessage)
			return
		}
		response := &bufferedResponse{header: make(http.Header)}
		next.ServeHTTP(response, r.WithContext(ctx))
		if ctx.Err() != nil {
			writeError(w, http.StatusGatewayTimeout, timeoutMessage)
			return
		}
		for key, values := range response.header {
			w.Header()[key] = values
		}
		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write(response.body.Bytes())
	})
}

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *bufferedResponse) Header() http.Header { return w.header }

func (w *bufferedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *bufferedResponse) Write(data []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.body.Write(data)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
