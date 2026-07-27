/*
Package mempool provides a thread-safe, in-memory waiting room for unconfirmed
transactions.

In a blockchain, transactions do not go directly into the permanent ledger.
They first sit in the Mempool until a Miner picks them up and packs them
into a Block. Only then are they considered "confirmed."

The Mempool uses a map keyed by transaction ID for O(1) lookups, and a
sync.RWMutex to safely handle concurrent reads and writes from multiple
goroutines (e.g., the API handler and gossip engine running simultaneously).
*/
package mempool

import (
	"p2pledger/internal/models"
	"sync"
)

// Mempool holds unconfirmed transactions waiting to be mined into a block.
type Mempool struct {
	mu           sync.RWMutex
	transactions map[string]models.Transaction // keyed by transaction ID for O(1) lookup
}

// NewMempool creates an empty Mempool ready to accept transactions.
func NewMempool() *Mempool {
	return &Mempool{
		transactions: make(map[string]models.Transaction),
	}
}

// Add inserts a transaction into the Mempool.
// Returns true if the transaction was added (it was new).
// Returns false if a transaction with the same ID already exists.
func (m *Mempool) Add(tx models.Transaction) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.transactions[tx.ID]; exists {
		return false
	}
	m.transactions[tx.ID] = tx
	return true
}

// Exists checks whether a transaction with the given ID is in the Mempool.
func (m *Mempool) Exists(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.transactions[id]
	return exists
}

// GetPending returns a copy of all unconfirmed transactions currently
// in the Mempool. The returned slice is safe to use without holding
// the lock.
func (m *Mempool) GetPending() []models.Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	txs := make([]models.Transaction, 0, len(m.transactions))
	for _, tx := range m.transactions {
		txs = append(txs, tx)
	}
	return txs
}

// Remove deletes a list of transaction IDs from the Mempool.
// This is called after a Miner packs the transactions into a Block
// — they are now confirmed and no longer need to sit in the waiting room.
func (m *Mempool) Remove(txIDs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range txIDs {
		delete(m.transactions, id)
	}
}

// Size returns the number of unconfirmed transactions in the Mempool.
func (m *Mempool) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.transactions)
}
