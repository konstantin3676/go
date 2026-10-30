package ledger

func Reset() {
	transactions = nil
	budgets = make(map[string]Budget)
}
