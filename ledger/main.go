package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
)

func main() {
	fmt.Println("Ledger service started")

	file, err := os.Open("budgets.json")
	if err != nil {
		log.Fatalf("could not open budgets file: %v", err)
	}
	defer file.Close()

	if err := LoadBudgets(bufio.NewReader(file)); err != nil {
		log.Fatalf("could not load budgets: %v", err)
	}

	testTransactions := []Transaction{
		{Amount: 1250.50, Category: "Продукты", Description: "Покупка продуктов", Date: "2026-10-01"},
		{Amount: 1000, Category: "Продукты", Description: "Большая закупка продуктов", Date: "2026-10-02"},
		{Amount: 350, Category: "Транспорт", Description: "Поездка на такси", Date: "2026-10-02"},
		{Amount: 2400, Category: "Образование", Description: "Книга по Go", Date: "2026-10-02"},
	}

	for _, tx := range testTransactions {
		if err := AddTransaction(tx); err != nil {
			if errors.Is(err, ErrBudgetExceeded) {
				fmt.Printf("Transaction rejected: %v\n", err)
				continue
			}
			log.Printf("could not add transaction: %v", err)
		}
	}

	fmt.Println("Transactions:")
	for _, tx := range ListTransactions() {
		fmt.Printf("ID: %d, amount: %.2f, category: %s, description: %s, date: %s\n",
			tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}
}
