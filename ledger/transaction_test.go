package ledger

import "testing"

func TestAddAndListTransactions(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	first := Transaction{Amount: 100, Category: "Продукты"}
	second := Transaction{Amount: 250, Category: "Транспорт"}

	if err := AddTransaction(first); err != nil {
		t.Fatalf("AddTransaction(first) returned an error: %v", err)
	}
	if err := AddTransaction(second); err != nil {
		t.Fatalf("AddTransaction(second) returned an error: %v", err)
	}

	got, err := ListTransactions()
	if err != nil {
		t.Fatalf("ListTransactions() returned an error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListTransactions() returned %d transactions, want 2", len(got))
	}
	if got[0].ID != 2 || got[1].ID != 1 {
		t.Fatalf("transaction IDs are %d and %d, want 2 and 1", got[0].ID, got[1].ID)
	}

	got[0].Amount = 999
	current, err := ListTransactions()
	if err != nil {
		t.Fatalf("ListTransactions() returned an error: %v", err)
	}
	if current[0].Amount != second.Amount {
		t.Fatal("ListTransactions() returned the internal storage instead of a copy")
	}
}

func TestAddTransactionRejectsZeroAmount(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	err := AddTransaction(Transaction{Amount: 0, Category: "Продукты"})
	if err == nil {
		t.Fatal("AddTransaction() returned nil for a zero amount")
	}
	transactions, err := ListTransactions()
	if err != nil {
		t.Fatalf("ListTransactions() returned an error: %v", err)
	}
	if len(transactions) != 0 {
		t.Fatal("AddTransaction() saved a transaction with a zero amount")
	}
}

func TestAddTransactionRejectsNegativeAmount(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	err := AddTransaction(Transaction{Amount: -50, Category: "Продукты"})
	if err == nil {
		t.Fatal("AddTransaction() returned nil for a negative amount")
	}
	transactions, err := ListTransactions()
	if err != nil {
		t.Fatalf("ListTransactions() returned an error: %v", err)
	}
	if len(transactions) != 0 {
		t.Fatal("AddTransaction() saved a transaction with a negative amount")
	}
}

func TestAddTransactionRejectsEmptyCategory(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	err := AddTransaction(Transaction{Amount: 100, Category: ""})
	if err == nil {
		t.Fatal("AddTransaction() returned nil for an empty category")
	}
	transactions, err := ListTransactions()
	if err != nil {
		t.Fatalf("ListTransactions() returned an error: %v", err)
	}
	if len(transactions) != 0 {
		t.Fatal("AddTransaction() saved a transaction with an empty category")
	}
}
