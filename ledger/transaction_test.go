package main

import "testing"

func TestAddAndListTransactions(t *testing.T) {
	transactions = nil
	budgets = map[string]Budget{}

	first := Transaction{Amount: 100, Category: "Продукты"}
	second := Transaction{Amount: 250, Category: "Транспорт"}

	if err := AddTransaction(first); err != nil {
		t.Fatalf("AddTransaction(first) returned an error: %v", err)
	}
	if err := AddTransaction(second); err != nil {
		t.Fatalf("AddTransaction(second) returned an error: %v", err)
	}

	got := ListTransactions()
	if len(got) != 2 {
		t.Fatalf("ListTransactions() returned %d transactions, want 2", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("transaction IDs are %d and %d, want 1 and 2", got[0].ID, got[1].ID)
	}

	got[0].Amount = 999
	if ListTransactions()[0].Amount != first.Amount {
		t.Fatal("ListTransactions() returned the internal storage instead of a copy")
	}
}

func TestAddTransactionRejectsZeroAmount(t *testing.T) {
	transactions = nil
	budgets = map[string]Budget{}

	err := AddTransaction(Transaction{Amount: 0, Category: "Продукты"})
	if err == nil {
		t.Fatal("AddTransaction() returned nil for a zero amount")
	}
	if len(ListTransactions()) != 0 {
		t.Fatal("AddTransaction() saved a transaction with a zero amount")
	}
}

func TestAddTransactionRejectsNegativeAmount(t *testing.T) {
	transactions = nil
	budgets = map[string]Budget{}

	err := AddTransaction(Transaction{Amount: -50, Category: "Продукты"})
	if err == nil {
		t.Fatal("AddTransaction() returned nil for a negative amount")
	}
	if len(ListTransactions()) != 0 {
		t.Fatal("AddTransaction() saved a transaction with a negative amount")
	}
}

func TestAddTransactionRejectsEmptyCategory(t *testing.T) {
	transactions = nil
	budgets = map[string]Budget{}

	err := AddTransaction(Transaction{Amount: 100, Category: ""})
	if err == nil {
		t.Fatal("AddTransaction() returned nil for an empty category")
	}
	if len(ListTransactions()) != 0 {
		t.Fatal("AddTransaction() saved a transaction with an empty category")
	}
}
