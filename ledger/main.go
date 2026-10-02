package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Ledger service started")

	testTransactions := []Transaction{
		{Amount: 1250.50, Category: "Продукты", Description: "Покупка продуктов", Date: "2026-10-01"},
		{Amount: 350, Category: "Транспорт", Description: "Поездка на такси", Date: "2026-10-02"},
		{Amount: 2400, Category: "Образование", Description: "Книга по Go", Date: "2026-10-02"},
	}

	for _, tx := range testTransactions {
		if err := AddTransaction(tx); err != nil {
			log.Printf("could not add transaction: %v", err)
		}
	}

	fmt.Println("Transactions:")
	for _, tx := range ListTransactions() {
		fmt.Printf("ID: %d, amount: %.2f, category: %s, description: %s, date: %s\n",
			tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}
}
