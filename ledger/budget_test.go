package main

import (
	"errors"
	"strings"
	"testing"
)

func TestSetBudgetAddsAndUpdatesBudget(t *testing.T) {
	budgets = map[string]Budget{}

	if err := SetBudget(Budget{Category: "Продукты", Limit: 1000}); err != nil {
		t.Fatalf("SetBudget() returned an error: %v", err)
	}
	if err := SetBudget(Budget{Category: "Продукты", Limit: 1500}); err != nil {
		t.Fatalf("SetBudget() returned an error: %v", err)
	}

	got, exists := budgets["Продукты"]
	if !exists {
		t.Fatal("SetBudget() did not add a budget")
	}
	if got.Limit != 1500 {
		t.Fatalf("budget limit is %.2f, want 1500", got.Limit)
	}
}

func TestSetBudgetRejectsZeroLimit(t *testing.T) {
	budgets = map[string]Budget{}

	err := SetBudget(Budget{Category: "Продукты", Limit: 0})
	if err == nil {
		t.Fatal("SetBudget() returned nil for a zero limit")
	}
	if _, exists := budgets["Продукты"]; exists {
		t.Fatal("SetBudget() saved a budget with zero limit")
	}
}

func TestSetBudgetRejectsNegativeLimit(t *testing.T) {
	budgets = map[string]Budget{}

	err := SetBudget(Budget{Category: "Продукты", Limit: -100})
	if err == nil {
		t.Fatal("SetBudget() returned nil for a negative limit")
	}
	if _, exists := budgets["Продукты"]; exists {
		t.Fatal("SetBudget() saved a budget with negative limit")
	}
}

func TestSetBudgetRejectsEmptyCategory(t *testing.T) {
	budgets = map[string]Budget{}

	err := SetBudget(Budget{Category: "", Limit: 1000})
	if err == nil {
		t.Fatal("SetBudget() returned nil for an empty category")
	}
	if len(budgets) != 0 {
		t.Fatal("SetBudget() saved a budget with empty category")
	}
}

func TestAddTransactionChecksBudget(t *testing.T) {
	transactions = nil
	budgets = map[string]Budget{}
	_ = SetBudget(Budget{Category: "Продукты", Limit: 500})

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

func TestLoadBudgetsRejectsInvalidBudget(t *testing.T) {
	budgets = map[string]Budget{}
	input := `[
		{"category": "", "limit": 1000}
	]`

	err := LoadBudgets(strings.NewReader(input))
	if err == nil {
		t.Fatal("LoadBudgets() returned nil for an invalid budget")
	}
	if !strings.Contains(err.Error(), "could not set budget") {
		t.Fatalf("LoadBudgets() returned an unclear error: %v", err)
	}
}

func TestCheckValid(t *testing.T) {
	if err := CheckValid(Transaction{Amount: 100, Category: "Продукты"}); err != nil {
		t.Fatalf("CheckValid(valid tx) returned an error: %v", err)
	}
	if err := CheckValid(Budget{Category: "Продукты", Limit: 500}); err != nil {
		t.Fatalf("CheckValid(valid budget) returned an error: %v", err)
	}
	if err := CheckValid(Transaction{Amount: 0, Category: "Продукты"}); err == nil {
		t.Fatal("CheckValid(zero amount) returned nil")
	}
	if err := CheckValid(Budget{Category: "", Limit: 0}); err == nil {
		t.Fatal("CheckValid(invalid budget) returned nil")
	}
}