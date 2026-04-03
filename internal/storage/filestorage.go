package storage

import "p2pledger/internal/models"

type FileStorage struct {
	FilePath string
}

func NewFileStorage(path string) *FileStorage {
	return &FileStorage{FilePath: path}
}

func (f *FileStorage) LoadTransactions() ([]models.Transaction, error) {
	// TODO: read from file
	
	return nil, nil
}

func (f *FileStorage) SaveTransaction(tx models.Transaction) error {
	// TODO:
	// 1. load existing
	// 2. append
	// 3. write back
	return nil
}

func (f *FileStorage) TransactionExists(id string) (bool, error) {
	// TODO: check in loaded transactions
	return false, nil
}