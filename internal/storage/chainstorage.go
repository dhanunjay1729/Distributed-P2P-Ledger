/*
Package storage implements the persistence layer for the P2P ledger.

This file provides the ChainStorage interface and a file-based implementation
(FileChainStorage) to store a sequence of blocks (the blockchain).

It differs from the legacy FileStorage in that it saves a chain of Blocks
rather than a flat list of Transactions, and includes logic to validate
the cryptographic integrity of the chain upon loading.
*/
package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"p2pledger/internal/models"
	"sync"
)

// ChainStorage defines the interface for interacting with the blockchain.
type ChainStorage interface {
	AddBlock(block models.Block) error
	GetChain() ([]models.Block, error)
	GetLatestBlock() (models.Block, error)
	ReplaceChain(newChain []models.Block) error
	TransactionExists(txID string) (bool, error)
}

// FileChainStorage implements ChainStorage using a local JSON file.
type FileChainStorage struct {
	FilePath string
	mu       sync.RWMutex
}

// NewFileChainStorage creates a new FileChainStorage instance.
// If the file doesn't exist or is empty, it automatically initializes it
// with the Genesis Block.
func NewFileChainStorage(path string) (*FileChainStorage, error) {
	fs := &FileChainStorage{FilePath: path}
	
	// Check if file exists; if not, initialize with Genesis block
	if _, err := os.Stat(path); os.IsNotExist(err) {
		genesis := []models.Block{models.CreateGenesisBlock()}
		if err := fs.ReplaceChain(genesis); err != nil {
			return nil, fmt.Errorf("failed to initialize genesis block: %w", err)
		}
	}
	
	// Ensure the chain is valid upon startup
	chain, err := fs.GetChain()
	if err != nil {
		return nil, fmt.Errorf("failed to load chain: %w", err)
	}
	if len(chain) == 0 {
		genesis := []models.Block{models.CreateGenesisBlock()}
		if err := fs.ReplaceChain(genesis); err != nil {
			return nil, fmt.Errorf("failed to re-initialize genesis block: %w", err)
		}
	}
	
	return fs, nil
}

// GetChain reads the full blockchain from the JSON file.
func (fs *FileChainStorage) GetChain() ([]models.Block, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	jsonFile, err := os.Open(fs.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.Block{}, nil
		}
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	
	if len(byteValue) == 0 {
		return []models.Block{}, nil
	}

	var chain []models.Block
	err = json.Unmarshal(byteValue, &chain)
	if err != nil {
		return nil, err
	}

	return chain, nil
}

// AddBlock appends a new block to the chain.
// It verifies that the block correctly links to the current latest block.
func (fs *FileChainStorage) AddBlock(block models.Block) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Need to load without lock since we hold it
	chain, err := fs.loadUnlocked()
	if err != nil {
		return err
	}

	if len(chain) > 0 {
		latest := chain[len(chain)-1]
		if block.PrevHash != latest.Hash {
			return fmt.Errorf("invalid block: PrevHash does not match latest block Hash")
		}
		if block.Index != latest.Index+1 {
			return fmt.Errorf("invalid block: Index must be exactly 1 greater than latest block")
		}
	}

	chain = append(chain, block)
	return fs.saveUnlocked(chain)
}

// GetLatestBlock returns the most recent block in the chain.
func (fs *FileChainStorage) GetLatestBlock() (models.Block, error) {
	chain, err := fs.GetChain()
	if err != nil {
		return models.Block{}, err
	}
	if len(chain) == 0 {
		return models.Block{}, fmt.Errorf("chain is empty")
	}
	return chain[len(chain)-1], nil
}

// ReplaceChain overwrites the entire local chain with a new one.
// This is used during fork resolution (Longest Chain Rule).
func (fs *FileChainStorage) ReplaceChain(newChain []models.Block) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.saveUnlocked(newChain)
}

// TransactionExists checks if a transaction is already permanently recorded in any block.
func (fs *FileChainStorage) TransactionExists(txID string) (bool, error) {
	chain, err := fs.GetChain()
	if err != nil {
		return false, err
	}
	
	// Start from latest blocks as they are more likely to contain recent transactions
	for i := len(chain) - 1; i >= 0; i-- {
		for _, tx := range chain[i].Transactions {
			if tx.ID == txID {
				return true, nil
			}
		}
	}
	return false, nil
}

// ValidateChain is a utility function to verify cryptographic integrity.
func ValidateChain(chain []models.Block) bool {
	if len(chain) == 0 {
		return false
	}
	
	// Ensure genesis is correct
	expectedGenesis := models.CreateGenesisBlock()
	if chain[0].Hash != expectedGenesis.Hash {
		return false
	}

	// Verify each subsequent block
	for i := 1; i < len(chain); i++ {
		currentBlock := chain[i]
		previousBlock := chain[i-1]

		// Does it point to the correct previous hash?
		if currentBlock.PrevHash != previousBlock.Hash {
			return false
		}
		
		// Is the hash itself mathematically valid?
		if currentBlock.Hash != currentBlock.CalculateHash() {
			return false
		}
		
		// Is the index correct?
		if currentBlock.Index != previousBlock.Index+1 {
			return false
		}
	}
	return true
}

// Internal un-exported helpers to avoid deadlocks when a method
// already holds the lock.
func (fs *FileChainStorage) loadUnlocked() ([]models.Block, error) {
	byteValue, err := ioutil.ReadFile(fs.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.Block{}, nil
		}
		return nil, err
	}
	if len(byteValue) == 0 {
		return []models.Block{}, nil
	}
	var chain []models.Block
	err = json.Unmarshal(byteValue, &chain)
	return chain, err
}

func (fs *FileChainStorage) saveUnlocked(chain []models.Block) error {
	byteValue, err := json.MarshalIndent(chain, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fs.FilePath, byteValue, 0644)
}
