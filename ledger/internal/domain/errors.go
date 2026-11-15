package domain

import "errors"

var (
	ErrBudgetExceeded = errors.New("budget exceeded")
	ErrBudgetNotFound = errors.New("budget not found")
)
