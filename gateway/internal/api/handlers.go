package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"ledger"
)

func HandleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tx := req.ToDomain()

	if err := tx.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := ledger.AddTransactionAndReturn(tx)
	if err != nil {
		if errors.Is(err, ledger.ErrBudgetExceeded) {
			writeError(w, http.StatusConflict, "budget exceeded")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, NewTransactionResponse(created))
}

func HandleListTransactions(w http.ResponseWriter, r *http.Request) {
	transactions, err := ledger.ListTransactions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]TransactionResponse, 0, len(transactions))
	for _, tx := range transactions {
		response = append(response, NewTransactionResponse(tx))
	}

	writeJSON(w, http.StatusOK, response)
}

func HandleCreateBudget(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	budget := req.ToDomain()

	if err := budget.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := ledger.SetBudget(budget); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, NewBudgetResponse(budget))
}

func HandleListBudgets(w http.ResponseWriter, r *http.Request) {
	budgets, err := ledger.ListBudgets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]BudgetResponse, 0, len(budgets))
	for _, b := range budgets {
		response = append(response, NewBudgetResponse(b))
	}

	writeJSON(w, http.StatusOK, response)
}

func HandleReportSummary(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "from and to query parameters are required")
		return
	}
	fromDate, fromErr := time.Parse("2006-01-02", from)
	toDate, toErr := time.Parse("2006-01-02", to)
	if fromErr != nil || toErr != nil || fromDate.After(toDate) {
		writeError(w, http.StatusBadRequest, "from and to must be valid dates with from <= to")
		return
	}

	report, err := ledger.GetReportSummary(r.Context(), from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}
