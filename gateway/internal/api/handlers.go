package api

import (
	"encoding/json"
	"errors"
	"net/http"

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

	if err := ledger.AddTransaction(tx); err != nil {
		if errors.Is(err, ledger.ErrBudgetExceeded) {
			writeError(w, http.StatusConflict, "budget exceeded")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	transactions := ledger.ListTransactions()
	created := transactions[len(transactions)-1]

	writeJSON(w, http.StatusCreated, NewTransactionResponse(created))
}

func HandleListTransactions(w http.ResponseWriter, r *http.Request) {
	transactions := ledger.ListTransactions()

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
	budgets := ledger.ListBudgets()

	response := make([]BudgetResponse, 0, len(budgets))
	for _, b := range budgets {
		response = append(response, NewBudgetResponse(b))
	}

	writeJSON(w, http.StatusOK, response)
}
