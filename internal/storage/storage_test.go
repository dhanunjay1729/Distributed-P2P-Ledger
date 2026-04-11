/*
 Test cases for the file-based storage implementation.
 This test file includes tests for saving and loading transactions, 
 as well as checking for transaction existence.
 what does it mean?
 This means that the test cases in this file are designed to verify the 
 functionality of the file-based storage system for transactions.
 The tests will ensure that transactions can be saved to a file,
 loaded back correctly, and that the system can check if a transaction with a 
 specific ID exists in the storage. These tests help confirm that the file
 storage implementation is working as intended and can reliably manage 
 transaction data on disk.
*/

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