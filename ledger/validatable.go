package main

import "fmt"

type Validatable interface {
	Validate() error
}

func CheckValid(v Validatable) error {
	if err := v.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}