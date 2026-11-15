package ledger

import "context"

func AddTransaction(tx Transaction) error {
	_, err := AddTransactionAndReturn(tx)
	return err
}

func AddTransactionAndReturn(tx Transaction) (Transaction, error) {
	return legacyService().AddTransaction(context.Background(), tx)
}

func ListTransactions() ([]Transaction, error) {
	return legacyService().ListTransactions(context.Background())
}
