package main

import (
	"errors"
	"strings"
	"testing"
)

func TestSetBudgetAddsAndUpdatesBudget(t *testing.T) {
	budgets = map[string]Budget{}

	SetBudget(Budget{Category: "Продукты", Limit: 1000})
	SetBudget(Budget{Category: "Продукты", Limit: 1500})

	got, exists := budgets["Продукты"]
	if !exists {
		t.Fatal("SetBudget() did not add a budget")
	}
	if got.Limit != 1500 {
		t.Fatalf("budget limit is %.2f, want 1500", got.Limit)
	}
}

func TestAddTransactionChecksBudget(t *testing.T) {
	transactions = nil
	budgets = map[string]Budget{}
	SetBudget(Budget{Category: "Продукты", Limit: 500})

	if err := AddTransaction(Transaction{Amount: 200, Category: "Продукты"}); err != nil {
		t.Fatalf("transaction within budget returned an error: %v", err)
	}
	if err := AddTransaction(Transaction{Amount: 300, Category: "Продукты"}); err != nil {
		t.Fatalf("transaction reaching budget exactly returned an error: %v", err)
	}

	err := AddTransaction(Transaction{Amount: 1, Category: "Продукты"})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("transaction exceeding budget returned %v, want ErrBudgetExceeded", err)
	}
	if len(ListTransactions()) != 2 {
		t.Fatal("transaction exceeding budget was saved")
	}
}

func TestAddTransactionWithoutBudget(t *testing.T) {
	transactions = nil
	budgets = map[string]Budget{}

	if err := AddTransaction(Transaction{Amount: 10000, Category: "Образование"}); err != nil {
		t.Fatalf("transaction without a budget returned an error: %v", err)
	}
}

func TestLoadBudgets(t *testing.T) {
	budgets = map[string]Budget{}
	input := `[
		{"category": "Продукты", "limit": 5000},
		{"category": "Транспорт", "limit": 1000}
	]`

	if err := LoadBudgets(strings.NewReader(input)); err != nil {
		t.Fatalf("LoadBudgets() returned an error: %v", err)
	}
	if len(budgets) != 2 {
		t.Fatalf("LoadBudgets() loaded %d budgets, want 2", len(budgets))
	}
	if budgets["Продукты"].Limit != 5000 {
		t.Fatalf("food budget is %.2f, want 5000", budgets["Продукты"].Limit)
	}
}

func TestLoadBudgetsRejectsInvalidJSON(t *testing.T) {
	err := LoadBudgets(strings.NewReader(`[{"category":`))
	if err == nil {
		t.Fatal("LoadBudgets() returned nil for invalid JSON")
	}
	if !strings.Contains(err.Error(), "could not parse budgets JSON") {
		t.Fatalf("LoadBudgets() returned an unclear error: %v", err)
	}
}
