package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"ledger"
)

const (
	defaultImportWorkers = 4
	maxImportWorkers     = 32
)

func (h *Handler) HandleImportTransactions(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid query parameters")
		return
	}
	workers := defaultImportWorkers
	if values, ok := query["workers"]; ok {
		workers, err = strconv.Atoi(values[0])
		if len(values) != 1 || err != nil || workers < 1 || workers > maxImportWorkers {
			writeError(w, http.StatusBadRequest, "workers must be a single integer from 1 to "+strconv.Itoa(maxImportWorkers))
			return
		}
	}

	var batch []*CreateTransactionRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&batch); err != nil || batch == nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
		writeError(w, http.StatusBadRequest, "expected a JSON array of transactions")
		return
	}
	transactions := make([]ledger.Transaction, len(batch))
	for i, req := range batch {
		if req == nil {
			writeError(w, http.StatusBadRequest, "expected a JSON array of transactions")
			return
		}
		transactions[i] = req.ToDomain()
	}

	summary, err := h.service.ImportTransactions(r.Context(), transactions, workers)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
