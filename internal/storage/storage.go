package storage

import "p2pledger/internal/models"

type Storage interface {
	LoadTransactions() ([]models.Transaction, error)
	SaveTransaction(tx models.Transaction) error
	TransactionExists(id string) (bool, error)
}