package ledger

func Reset() {
	stateMu.Lock()
	transactions = nil
	budgets = make(map[string]Budget)
	stateMu.Unlock()

	if DB != nil {
		_, _ = DB.Exec(`TRUNCATE TABLE expenses, budgets RESTART IDENTITY`)
	}
}
