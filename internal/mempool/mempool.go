// Package mempool holds a bounded, thread-safe queue of unconfirmed messages.
package mempool

import (
	"errors"
	"p2pledger/internal/models"
	"sort"
	"sync"
)

const MaxPending = 1000

var (
	ErrDuplicate = errors.New("transaction already known")
	ErrFull      = errors.New("mempool capacity reached; retry later")
)

type Mempool struct {
	mu           sync.RWMutex
	transactions map[string]models.Transaction
}

func NewMempool() *Mempool { return &Mempool{transactions: make(map[string]models.Transaction)} }
func (m *Mempool) TryAdd(tx models.Transaction) error {
	if err := models.ValidateTransaction(tx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.transactions[tx.ID]; ok {
		return ErrDuplicate
	}
	if len(m.transactions) >= MaxPending {
		return ErrFull
	}
	m.transactions[tx.ID] = tx
	return nil
}
func (m *Mempool) Add(tx models.Transaction) bool { return m.TryAdd(tx) == nil }
func (m *Mempool) Exists(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.transactions[id]
	return ok
}
func (m *Mempool) GetPending() []models.Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]models.Transaction, 0, len(m.transactions))
	for _, tx := range m.transactions {
		out = append(out, tx)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (m *Mempool) Remove(ids []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		delete(m.transactions, id)
	}
}
func (m *Mempool) Size() int { m.mu.RLock(); defer m.mu.RUnlock(); return len(m.transactions) }

// PrepareReorg has no side effects. Refuse an over-capacity reorganization rather
// than silently dropping unconfirmed data. The engine serializes Prepare/Replace.
func (m *Mempool) PrepareReorg(old, newChain []models.Block) ([]models.Transaction, error) {
	confirmed := make(map[string]bool)
	for _, b := range newChain {
		for _, tx := range b.Transactions {
			confirmed[tx.ID] = true
		}
	}
	next := make(map[string]models.Transaction)
	for _, tx := range m.GetPending() {
		if !confirmed[tx.ID] {
			next[tx.ID] = tx
		}
	}
	for _, b := range old {
		for _, tx := range b.Transactions {
			if !confirmed[tx.ID] {
				next[tx.ID] = tx
			}
		}
	}
	if len(next) > MaxPending {
		return nil, ErrFull
	}
	out := make([]models.Transaction, 0, len(next))
	for _, tx := range next {
		out = append(out, tx)
	}
	return out, nil
}
func (m *Mempool) Replace(txs []models.Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions = make(map[string]models.Transaction, len(txs))
	for _, tx := range txs {
		m.transactions[tx.ID] = tx
	}
}
