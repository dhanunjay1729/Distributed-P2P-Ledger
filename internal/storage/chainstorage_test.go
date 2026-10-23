package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"p2pledger/internal/models"
	"p2pledger/internal/testutil"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *FileChainStorage {
	t.Helper()
	s, err := NewFileChainStorage(filepath.Join(t.TempDir(), "nested", "chain.json"), 1)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPersistenceAndValidation(t *testing.T) {
	s := newStore(t)
	g, _ := s.GetLatestBlock()
	b := testutil.Block(g, "tx")
	bad := b
	bad.Hash = "x"
	if s.AddBlock(bad) == nil {
		t.Fatal("forged block accepted")
	}
	if err := s.AddBlock(b); err != nil {
		t.Fatal(err)
	}
	if err := s.AddBlock(testutil.Block(b, "tx")); err == nil {
		t.Fatal("confirmed transaction replay accepted")
	}
	reloaded, err := NewFileChainStorage(s.FilePath, 1)
	if err != nil {
		t.Fatal(err)
	}
	tip, _ := reloaded.GetLatestBlock()
	if tip.Hash != b.Hash {
		t.Fatal("restart lost chain")
	}
	c, _ := s.GetChain()
	c[1].Transactions[0].Data = "external mutation"
	tip, _ = s.GetLatestBlock()
	if tip.Transactions[0].Data != b.Transactions[0].Data {
		t.Fatal("snapshot aliases live state")
	}
}

func TestStartupPreservesInvalidFiles(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte("{bad"), []byte("[]"), []byte("null")} {
		p := filepath.Join(t.TempDir(), "chain.json")
		if err := os.WriteFile(p, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := NewFileChainStorage(p, 1); err == nil {
			t.Fatal("invalid file accepted")
		}
		after, _ := os.ReadFile(p)
		if !bytes.Equal(raw, after) {
			t.Fatal("invalid file overwritten")
		}
	}
	s := newStore(t)
	c, _ := s.GetChain()
	c[0].Timestamp++
	raw, _ := json.Marshal(c)
	os.WriteFile(s.FilePath, raw, 0600)
	if _, err := NewFileChainStorage(s.FilePath, 1); err == nil {
		t.Fatal("tampered genesis accepted")
	}
}

func TestReplaceCannotRollbackLiveProgress(t *testing.T) {
	s := newStore(t)
	g, _ := s.GetLatestBlock()
	candidate := []models.Block{g, testutil.Block(g, "peer")}
	b1 := testutil.Block(g, "local1")
	b2 := testutil.Block(b1, "local2")
	if err := s.AddBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := s.AddBlock(b2); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(s.ReplaceChain(candidate), ErrNotBetter) {
		t.Fatal("stale candidate rolled back newer local chain")
	}
	tip, _ := s.GetLatestBlock()
	if tip.Hash != b2.Hash {
		t.Fatal("tip changed")
	}
}

func TestEqualLengthForkHasStableWinner(t *testing.T) {
	s := newStore(t)
	g, _ := s.GetLatestBlock()
	a := testutil.Block(g, "a")
	b := testutil.Block(g, "b")
	if a.Hash > b.Hash {
		a, b = b, a
	}
	if err := s.AddBlock(b); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceChain([]models.Block{g, a}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(s.ReplaceChain([]models.Block{g, b}), ErrNotBetter) {
		t.Fatal("tie-break oscillates")
	}
}

func TestFailedSaveKeepsCommittedChain(t *testing.T) {
	s := newStore(t)
	g, _ := s.GetLatestBlock()
	raw, _ := os.ReadFile(s.FilePath)
	path := s.FilePath
	s.FilePath = filepath.Join(t.TempDir(), "missing", "chain.json")
	if err := s.AddBlock(testutil.Block(g, "tx")); err == nil {
		t.Fatal("expected save failure")
	}
	s.FilePath = path
	after, _ := os.ReadFile(path)
	tip, _ := s.GetLatestBlock()
	if !bytes.Equal(raw, after) || tip.Hash != g.Hash {
		t.Fatal("failed save changed committed state")
	}
}
