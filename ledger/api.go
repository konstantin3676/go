package ledger

import (
	"ledger/internal/domain"
	"ledger/internal/service"
)

type (
	Budget          = domain.Budget
	Transaction     = domain.Transaction
	ReportSummary   = domain.ReportSummary
	Validatable     = domain.Validatable
	Service         = service.Service
	ValidationError = service.ValidationError
)

var (
	ErrBudgetExceeded = domain.ErrBudgetExceeded
	ErrBudgetNotFound = domain.ErrBudgetNotFound
)
