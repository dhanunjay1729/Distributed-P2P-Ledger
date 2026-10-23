package mempool

import (
	"errors"
	"fmt"
	"p2pledger/internal/models"
	"p2pledger/internal/testutil"
	"testing"
)

func TestCapacityDedupAndDeterministicOrder(t *testing.T) {
	m := NewMempool()
	for i := 0; i < MaxPending; i++ {
		if err := m.TryAdd(models.Transaction{ID: fmt.Sprintf("%04d", i), Data: "x", Timestamp: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(m.TryAdd(models.Transaction{ID: "new", Data: "x", Timestamp: 1}), ErrFull) {
		t.Fatal("unbounded queue")
	}
	if !errors.Is(m.TryAdd(models.Transaction{ID: "0000", Data: "changed", Timestamp: 1}), ErrDuplicate) {
		t.Fatal("duplicate overwritten")
	}
	txs := m.GetPending()
	if txs[0].ID != "0000" || txs[0].Data != "x" {
		t.Fatal("unstable snapshot")
	}
}
func TestReorgRestoresOrphansAndRemovesConfirmed(t *testing.T) {
	m := NewMempool()
	m.Add(models.Transaction{ID: "new", Data: "x", Timestamp: 1})
	g := models.CreateGenesisBlock(1)
	old := []models.Block{g, testutil.Block(g, "orphan")}
	next := []models.Block{g, testutil.Block(g, "new")}
	txs, err := m.PrepareReorg(old, next)
	if err != nil {
		t.Fatal(err)
	}
	m.Replace(txs)
	if !m.Exists("orphan") || m.Exists("new") {
		t.Fatal("incorrect reconciliation")
	}
	if err := m.TryAdd(models.Transaction{ID: "other", Data: "x", Timestamp: 1}); err != nil {
		t.Fatal(err)
	}
}
