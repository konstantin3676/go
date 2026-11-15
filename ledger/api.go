package ledger

import "ledger/internal/domain"

type (
	Budget        = domain.Budget
	Transaction   = domain.Transaction
	ReportSummary = domain.ReportSummary
	Validatable   = domain.Validatable
)

var (
	ErrBudgetExceeded = domain.ErrBudgetExceeded
	ErrBudgetNotFound = domain.ErrBudgetNotFound
)
