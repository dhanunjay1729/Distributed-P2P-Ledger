package storage

import (
	"os"
	"testing"
	"p2pledger/internal/models"
)

func setupTestFile(path string) {
	os.Remove(path) // clean previous test file
}

func TestSaveAndLoadTransactions(t *testing.T) {
	path := "test_ledger.json"
	setupTestFile(path)

	store := NewFileStorage(path)

	tx := models.Transaction{
		ID:        "1",
		Data:      "test data",
		Timestamp: 123456,
	}

	err := store.SaveTransaction(tx)
	if err != nil {
		t.Fatalf("failed to save transaction: %v", err)
	}

	txs, err := store.LoadTransactions()
	if err != nil {
		t.Fatalf("failed to load transactions: %v", err)
	}

	if len(txs) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txs))
	}

	if txs[0].ID != "1" {
		t.Fatalf("wrong transaction ID")
	}
}

func TestTransactionExists(t *testing.T) {
	path := "test_ledger.json"
	setupTestFile(path)

	store := NewFileStorage(path)

	tx := models.Transaction{
		ID:        "abc123",
		Data:      "check exists",
		Timestamp: 999,
	}

	store.SaveTransaction(tx)

	exists, err := store.TransactionExists("abc123")
	if err != nil {
		t.Fatalf("error checking existence: %v", err)
	}

	if !exists {
		t.Fatalf("transaction should exist")
	}

	exists, _ = store.TransactionExists("not_present")
	if exists {
		t.Fatalf("transaction should NOT exist")
	}
}