// Package memory provides isolated in-memory repositories for tests.
// Production uses PostgreSQL and does not fall back to in-memory storage.
package memory

import (
	"context"
	"sort"
	"sync"

	"ledger/internal/domain"
)

type Store struct {
	mu           sync.RWMutex
	budgets      map[string]domain.Budget
	transactions []domain.Transaction
}

func NewStore() *Store {
	return &Store{budgets: make(map[string]domain.Budget)}
}

type BudgetRepository struct {
	store *Store
}

var _ domain.BudgetRepository = (*BudgetRepository)(nil)

func NewBudgetRepository(store *Store) *BudgetRepository {
	return &BudgetRepository{store: store}
}

func (r *BudgetRepository) Upsert(ctx context.Context, budget domain.Budget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.budgets[budget.Category] = budget
	return nil
}

func (r *BudgetRepository) GetByCategory(ctx context.Context, category string) (domain.Budget, error) {
	if err := ctx.Err(); err != nil {
		return domain.Budget{}, err
	}
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()
	budget, found := r.store.budgets[category]
	if !found {
		return domain.Budget{}, domain.ErrBudgetNotFound
	}
	return budget, nil
}

func (r *BudgetRepository) List(ctx context.Context) ([]domain.Budget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.store.mu.RLock()
	result := make([]domain.Budget, 0, len(r.store.budgets))
	for _, budget := range r.store.budgets {
		result = append(result, budget)
	}
	r.store.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].Category < result[j].Category })
	return result, nil
}

type TransactionRepository struct {
	store *Store
}

var _ domain.TransactionRepository = (*TransactionRepository)(nil)

func NewTransactionRepository(store *Store) *TransactionRepository {
	return &TransactionRepository{store: store}
}

func (r *TransactionRepository) Add(ctx context.Context, tx domain.Transaction) (domain.Transaction, error) {
	if err := ctx.Err(); err != nil {
		return domain.Transaction{}, err
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	tx.ID = len(r.store.transactions) + 1
	r.store.transactions = append(r.store.transactions, tx)
	return tx, nil
}

func (r *TransactionRepository) List(ctx context.Context) ([]domain.Transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.store.mu.RLock()
	result := make([]domain.Transaction, len(r.store.transactions))
	copy(result, r.store.transactions)
	r.store.mu.RUnlock()
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Date == result[j].Date {
			return result[i].ID > result[j].ID
		}
		return result[i].Date > result[j].Date
	})
	return result, nil
}

func (r *TransactionRepository) SumByCategory(ctx context.Context, category string) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()
	var sum float64
	for _, tx := range r.store.transactions {
		if tx.Category == category {
			sum += tx.Amount
		}
	}
	return sum, nil
}
