/*
Package models defines the core data structures for the P2P blockchain ledger.

Block represents a single block in the blockchain. Each block contains:
  - Index: the block's position in the chain (0 for genesis, 1, 2, 3...)
  - Timestamp: Unix timestamp of when the block was mined
  - Transactions: the batch of transactions packed into this block
  - PrevHash: the SHA-256 hash of the previous block (creates the "chain")
  - Hash: this block's own SHA-256 hash (its unique fingerprint)
  - Nonce: the number that was guessed during Proof of Work mining

The CalculateHash method produces a deterministic SHA-256 fingerprint of the
block's contents. If even a single character in any field changes, the hash
changes completely, making tampering immediately detectable.

CreateGenesisBlock produces the very first block in the chain (Block #0).
It has no previous hash and contains no transactions.
*/
package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Block represents a single block in the blockchain.
type Block struct {
	Index        int           `json:"index"`         // Position in the chain (0, 1, 2, ...)
	Timestamp    int64         `json:"timestamp"`     // Unix timestamp of when this block was mined
	Transactions []Transaction `json:"transactions"`  // Batch of transactions packed into this block
	PrevHash     string        `json:"prev_hash"`     // SHA-256 hash of the previous block
	Hash         string        `json:"hash"`          // This block's own SHA-256 hash
	Nonce        int           `json:"nonce"`         // The number guessed during Proof of Work
}

// CalculateHash computes the SHA-256 hash of the block's contents.
//
// It concatenates: Index + Timestamp + all transaction IDs + PrevHash + Nonce
// into a single string, then runs SHA-256 on it.
//
// Why include transaction IDs? If someone swaps out a transaction inside the
// block, the hash will change, and the chain breaks.
//
// Why include PrevHash? It chains this block to the previous one. Changing
// any older block would cascade hash mismatches forward through every
// subsequent block.
func (b *Block) CalculateHash() string {
	// Collect all transaction IDs into a single string for hashing.
	txIDs := make([]string, len(b.Transactions))
	for i, tx := range b.Transactions {
		txIDs[i] = tx.ID
	}

	// Build the raw string that will be hashed.
	record := fmt.Sprintf("%d%d%s%s%d",
		b.Index,
		b.Timestamp,
		strings.Join(txIDs, ","),
		b.PrevHash,
		b.Nonce,
	)

	// Compute SHA-256 hash and return as a hex string.
	hash := sha256.Sum256([]byte(record))
	return hex.EncodeToString(hash[:])
}

// CreateGenesisBlock creates the very first block in the blockchain (Block #0).
//
// The genesis block is special:
//   - Index is 0
//   - PrevHash is empty (there is no block before it)
//   - It contains no transactions
//   - Nonce is 0 (no mining required for genesis)
//
// Every node in the network must start with the exact same genesis block,
// otherwise their chains will never match.
func CreateGenesisBlock() Block {
	genesis := Block{
		Index:        0,
		Timestamp:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		Transactions: []Transaction{},
		PrevHash:     "",
		Nonce:        0,
	}
	genesis.Hash = genesis.CalculateHash()
	return genesis
}
