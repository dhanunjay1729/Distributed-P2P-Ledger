// Package storage defines the Storage interface for managing transactions in the P2P ledger application. It provides methods for loading, saving, and checking the existence of transactions. This abstraction allows for different storage implementations (e.g., in-memory, file-based, database) to be used interchangeably without affecting the rest of the application logic.
package storage

import "p2pledger/internal/models"

type Storage interface {
	LoadTransactions() ([]models.Transaction, error)
	SaveTransaction(tx models.Transaction) error
	TransactionExists(id string) (bool, error)
}