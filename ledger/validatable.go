package ledger

import "ledger/internal/service"

func CheckValid(v Validatable) error {
	return service.CheckValid(v)
}
