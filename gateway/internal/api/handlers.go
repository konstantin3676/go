package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"ledger"
)

type Handler struct {
	service ledger.Service
}

func NewHandler(service ledger.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/transactions", h.HandleCreateTransaction)
	mux.HandleFunc("POST /api/transactions/bulk", h.HandleImportTransactions)
	mux.HandleFunc("GET /api/transactions", h.HandleListTransactions)
	mux.HandleFunc("POST /api/budgets", h.HandleCreateBudget)
	mux.HandleFunc("GET /api/budgets", h.HandleListBudgets)
	mux.HandleFunc("GET /api/reports/summary", h.HandleReportSummary)
	return mux
}

func (h *Handler) HandleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.service.AddTransaction(r.Context(), req.ToDomain())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, NewTransactionResponse(created))
}

func (h *Handler) HandleListTransactions(w http.ResponseWriter, r *http.Request) {
	transactions, err := h.service.ListTransactions(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	response := make([]TransactionResponse, 0, len(transactions))
	for _, tx := range transactions {
		response = append(response, NewTransactionResponse(tx))
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) HandleCreateBudget(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	budget := req.ToDomain()

	if err := h.service.SetBudget(r.Context(), budget); err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, NewBudgetResponse(budget))
}

func (h *Handler) HandleListBudgets(w http.ResponseWriter, r *http.Request) {
	budgets, err := h.service.ListBudgets(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}

	response := make([]BudgetResponse, 0, len(budgets))
	for _, b := range budgets {
		response = append(response, NewBudgetResponse(b))
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) HandleReportSummary(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "from and to query parameters are required")
		return
	}
	fromDate, fromErr := time.Parse(time.DateOnly, from)
	toDate, toErr := time.Parse(time.DateOnly, to)
	if fromErr != nil || toErr != nil || fromDate.After(toDate) {
		writeError(w, http.StatusBadRequest, "from and to must be valid dates with from <= to")
		return
	}
	report, err := h.service.GetReportSummary(r.Context(), from, to)
	if err != nil {
		var validationErr *ledger.ValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, "from and to must be valid dates with from <= to")
			return
		}
		writeServiceError(w, err)
		return
	}

	summary := make(map[string]float64, len(report))
	for _, item := range report {
		summary[item.Category] = item.Amount
	}
	writeJSON(w, http.StatusOK, summary)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var validationErr *ledger.ValidationError
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, http.StatusGatewayTimeout, timeoutMessage)
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ledger.ErrBudgetExceeded):
		writeError(w, http.StatusConflict, "budget exceeded")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
