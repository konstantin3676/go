package ledger

func Reset() {
	legacyStore.Reset()

	if DB != nil {
		_, _ = DB.Exec(`TRUNCATE TABLE expenses, budgets RESTART IDENTITY`)
	}
}
