// Package models defines the versioned rules shared by every node.
package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	ProtocolVersion      = 1
	DefaultDifficulty    = 2
	MaxDifficulty        = 4
	MaxBlockTransactions = 100
	MaxChainBlocks       = 10000
)

type Block struct {
	Version      int           `json:"version"`
	Difficulty   int           `json:"difficulty"`
	Index        int           `json:"index"`
	Timestamp    int64         `json:"timestamp"`
	Transactions []Transaction `json:"transactions"`
	PrevHash     string        `json:"prev_hash"`
	Hash         string        `json:"hash"`
	Nonce        uint64        `json:"nonce"`
}

// JSON boundaries prevent ambiguities such as ["a,b"] versus ["a","b"].
// Every transaction field is protected, not just the transaction ID.
func (b Block) CalculateHash() string {
	header := struct {
		Version      int           `json:"version"`
		Difficulty   int           `json:"difficulty"`
		Index        int           `json:"index"`
		Timestamp    int64         `json:"timestamp"`
		Transactions []Transaction `json:"transactions"`
		PrevHash     string        `json:"prev_hash"`
		Nonce        uint64        `json:"nonce"`
	}{b.Version, b.Difficulty, b.Index, b.Timestamp, b.Transactions, b.PrevHash, b.Nonce}
	raw, _ := json.Marshal(header)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func CheckDifficulty(d int) error {
	if d < 1 || d > MaxDifficulty {
		return fmt.Errorf("difficulty must be between 1 and %d", MaxDifficulty)
	}
	return nil
}

func CreateGenesisBlock(ds ...int) Block {
	d := DefaultDifficulty
	if len(ds) > 0 {
		d = ds[0]
	}
	b := Block{Version: ProtocolVersion, Difficulty: d, Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), Transactions: []Transaction{}}
	b.Hash = b.CalculateHash()
	return b
}

// ValidateBlock also works when a block's parent has not arrived yet.
func ValidateBlock(b Block, d int) error {
	if err := CheckDifficulty(d); err != nil {
		return err
	}
	if b.Version != ProtocolVersion || b.Difficulty != d {
		return fmt.Errorf("incompatible protocol version or difficulty")
	}
	if b.Index < 1 || b.Index >= MaxChainBlocks || len(b.PrevHash) != 64 {
		return fmt.Errorf("invalid block index or previous hash")
	}
	if b.Timestamp < CreateGenesisBlock(d).Timestamp || b.Timestamp > time.Now().Add(2*time.Minute).Unix() {
		return fmt.Errorf("invalid block timestamp")
	}
	if len(b.Transactions) == 0 || len(b.Transactions) > MaxBlockTransactions {
		return fmt.Errorf("block must contain 1 to %d transactions", MaxBlockTransactions)
	}
	seen := make(map[string]bool, len(b.Transactions))
	for _, tx := range b.Transactions {
		if err := ValidateTransaction(tx); err != nil {
			return err
		}
		if seen[tx.ID] {
			return fmt.Errorf("duplicate transaction %q in block", tx.ID)
		}
		seen[tx.ID] = true
	}
	if len(b.Hash) != 64 || b.Hash != b.CalculateHash() {
		return fmt.Errorf("invalid block hash")
	}
	if !strings.HasPrefix(b.Hash, strings.Repeat("0", d)) {
		return fmt.Errorf("block does not meet proof of work")
	}
	return nil
}

func ValidateChain(chain []Block, d int) error {
	if err := CheckDifficulty(d); err != nil {
		return err
	}
	if len(chain) == 0 || len(chain) > MaxChainBlocks {
		return fmt.Errorf("invalid chain length")
	}
	if !reflect.DeepEqual(chain[0], CreateGenesisBlock(d)) {
		return fmt.Errorf("invalid genesis block; legacy chains require a fresh data directory")
	}
	seen := make(map[string]bool)
	for i := 1; i < len(chain); i++ {
		b, prev := chain[i], chain[i-1]
		if err := ValidateBlock(b, d); err != nil {
			return fmt.Errorf("block %d: %w", i, err)
		}
		if b.Index != i || b.PrevHash != prev.Hash || b.Timestamp < prev.Timestamp {
			return fmt.Errorf("block %d does not follow its parent", i)
		}
		for _, tx := range b.Transactions {
			if seen[tx.ID] {
				return fmt.Errorf("transaction %q appears more than once", tx.ID)
			}
			seen[tx.ID] = true
		}
	}
	return nil
}

// Length represents work because every block has the same configured target.
// The smaller tip hash breaks equal-length ties so idle forks also converge.
func BetterChain(candidate, current []Block) bool {
	if len(candidate) != len(current) {
		return len(candidate) > len(current)
	}
	return len(candidate) > 0 && candidate[len(candidate)-1].Hash < current[len(current)-1].Hash
}
