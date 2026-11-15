package ledger

import "fmt"

func CheckValid(v Validatable) error {
	if err := v.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}
