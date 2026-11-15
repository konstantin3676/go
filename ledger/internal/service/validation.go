package service

import (
	"fmt"

	"ledger/internal/domain"
)

type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string {
	return e.Err.Error()
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

func CheckValid(v domain.Validatable) error {
	if err := v.Validate(); err != nil {
		return &ValidationError{Err: fmt.Errorf("validation failed: %w", err)}
	}
	return nil
}
