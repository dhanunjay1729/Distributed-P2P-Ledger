package storage

import (
	"os"
	"p2pledger/internal/models"
	"testing"
	"time"
)

func TestFileChainStorage(t *testing.T) {
	path := "test_chain.json"
	defer os.Remove(path)

	// 1. Test Initialization (Genesis block auto-creation)
	store, err := NewFileChainStorage(path)
	if err != nil {
		t.Fatalf("failed to initialize chain storage: %v", err)
	}

	chain, err := store.GetChain()
	if err != nil {
		t.Fatalf("failed to get chain: %v", err)
	}

	if len(chain) != 1 {
		t.Fatalf("expected chain length 1 (genesis), got %d", len(chain))
	}

	latest, err := store.GetLatestBlock()
	if err != nil {
		t.Fatalf("failed to get latest block: %v", err)
	}

	if latest.Index != 0 {
		t.Fatalf("expected genesis block index 0, got %d", latest.Index)
	}

	// 2. Test Adding a valid block
	tx := models.Transaction{ID: "tx1", Data: "test", Timestamp: time.Now().Unix()}
	newBlock := models.Block{
		Index:        1,
		Timestamp:    time.Now().Unix(),
		Transactions: []models.Transaction{tx},
		PrevHash:     latest.Hash,
		Nonce:        123,
	}
	newBlock.Hash = newBlock.CalculateHash()

	err = store.AddBlock(newBlock)
	if err != nil {
		t.Fatalf("failed to add valid block: %v", err)
	}

	chain, _ = store.GetChain()
	if len(chain) != 2 {
		t.Fatalf("expected chain length 2, got %d", len(chain))
	}

	// 3. Test adding an invalid block (wrong PrevHash)
	badBlock := models.Block{
		Index:    2,
		PrevHash: "fake_hash",
	}
	err = store.AddBlock(badBlock)
	if err == nil {
		t.Fatalf("expected error when adding block with invalid PrevHash")
	}
}

func TestValidateChain(t *testing.T) {
	genesis := models.CreateGenesisBlock()

	block1 := models.Block{
		Index:        1,
		Timestamp:    time.Now().Unix(),
		Transactions: []models.Transaction{},
		PrevHash:     genesis.Hash,
		Nonce:        0,
	}
	block1.Hash = block1.CalculateHash()

	chain := []models.Block{genesis, block1}

	if !ValidateChain(chain) {
		t.Fatalf("expected chain to be valid")
	}

	// Tamper with the chain
	chain[1].Transactions = []models.Transaction{{ID: "fake"}}
	if ValidateChain(chain) {
		t.Fatalf("expected tampered chain to be invalid")
	}
}
