// Package storage validates and atomically persists confirmed blocks.
package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"p2pledger/internal/models"
	"path/filepath"
	"sync"
)

const MaxChainBytes = 64 << 20

var (
	ErrNotBetter  = errors.New("candidate chain is not preferred over the current chain")
	ErrStaleBlock = errors.New("block does not extend the current tip")
)

type ChainStorage interface {
	AddBlock(models.Block) error
	GetChain() ([]models.Block, error)
	GetLatestBlock() (models.Block, error)
	ReplaceChain([]models.Block) error
	TransactionExists(string) (bool, error)
	Difficulty() int
}

type FileChainStorage struct {
	FilePath   string
	mu         sync.RWMutex
	chain      []models.Block
	txIDs      map[string]bool
	difficulty int
}

func NewFileChainStorage(path string, ds ...int) (*FileChainStorage, error) {
	d := models.DefaultDifficulty
	if len(ds) > 0 {
		d = ds[0]
	}
	if err := models.CheckDifficulty(d); err != nil {
		return nil, err
	}
	fs := &FileChainStorage{FilePath: path, difficulty: d}
	info, err := os.Stat(path)
	if err == nil && (info.Size() == 0 || info.Size() > MaxChainBytes) {
		return nil, fmt.Errorf("invalid chain file size; original file preserved")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		fs.chain = []models.Block{models.CreateGenesisBlock(d)}
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return nil, err
		}
		if err := fs.saveUnlocked(fs.chain); err != nil {
			return nil, err
		}
	} else {
		if err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&fs.chain); err != nil {
			return nil, fmt.Errorf("load chain: %w", err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return nil, fmt.Errorf("unexpected trailing chain data")
		}
		if err := models.ValidateChain(fs.chain, d); err != nil {
			return nil, fmt.Errorf("load chain: %w", err)
		}
	}
	fs.indexTransactions()
	return fs, nil
}

func (fs *FileChainStorage) Difficulty() int { return fs.difficulty }

func cloneChain(chain []models.Block) []models.Block {
	out := make([]models.Block, len(chain))
	copy(out, chain)
	for i := range out {
		out[i].Transactions = append([]models.Transaction{}, chain[i].Transactions...)
	}
	return out
}

func (fs *FileChainStorage) GetChain() ([]models.Block, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return cloneChain(fs.chain), nil
}

func (fs *FileChainStorage) GetLatestBlock() (models.Block, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	b := fs.chain[len(fs.chain)-1]
	b.Transactions = append([]models.Transaction{}, b.Transactions...)
	return b, nil
}

func (fs *FileChainStorage) AddBlock(b models.Block) error {
	if err := models.ValidateBlock(b, fs.difficulty); err != nil {
		return err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	prev := fs.chain[len(fs.chain)-1]
	if b.Index != prev.Index+1 || b.PrevHash != prev.Hash {
		return ErrStaleBlock
	}
	if b.Timestamp < prev.Timestamp {
		return fmt.Errorf("block timestamp precedes parent")
	}
	for _, tx := range b.Transactions {
		if fs.txIDs[tx.ID] {
			return fmt.Errorf("transaction %q already confirmed", tx.ID)
		}
	}
	candidate := append(cloneChain(fs.chain), b)
	if err := fs.saveUnlocked(candidate); err != nil {
		return err
	}
	fs.chain = cloneChain(candidate)
	for _, tx := range b.Transactions {
		fs.txIDs[tx.ID] = true
	}
	return nil
}

// Compare against the LIVE chain while holding the same lock as the replacement.
func (fs *FileChainStorage) ReplaceChain(chain []models.Block) error {
	candidate := cloneChain(chain)
	if err := models.ValidateChain(candidate, fs.difficulty); err != nil {
		return err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if !models.BetterChain(candidate, fs.chain) {
		return ErrNotBetter
	}
	if err := fs.saveUnlocked(candidate); err != nil {
		return err
	}
	fs.chain = candidate
	fs.indexTransactions()
	return nil
}

func (fs *FileChainStorage) TransactionExists(id string) (bool, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.txIDs[id], nil
}

func (fs *FileChainStorage) indexTransactions() {
	fs.txIDs = make(map[string]bool)
	for _, b := range fs.chain {
		for _, tx := range b.Transactions {
			fs.txIDs[tx.ID] = true
		}
	}
}

func ValidateChain(chain []models.Block, ds ...int) bool {
	d := models.DefaultDifficulty
	if len(ds) > 0 {
		d = ds[0]
	}
	return models.ValidateChain(chain, d) == nil
}

func (fs *FileChainStorage) saveUnlocked(chain []models.Block) error {
	raw, err := json.Marshal(chain)
	if err != nil {
		return err
	}
	if len(raw) > MaxChainBytes {
		return fmt.Errorf("chain exceeds %d bytes", MaxChainBytes)
	}
	f, err := os.CreateTemp(filepath.Dir(fs.FilePath), ".chain-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), fs.FilePath); err != nil {
		return err
	}
	// Once renamed, the commit succeeded. Directory sync improves crash durability.
	if dir, err := os.Open(filepath.Dir(fs.FilePath)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
