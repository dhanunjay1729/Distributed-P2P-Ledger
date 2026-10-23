package miner

import (
	"context"
	"errors"
	gossip "p2pledger/Gossip_Engine"
	"p2pledger/internal/mempool"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
	"path/filepath"
	"testing"
	"time"
)

func TestMineOnlyPendingAndRemoveConfirmed(t *testing.T) {
	cs, err := storage.NewFileChainStorage(filepath.Join(t.TempDir(), "chain.json"), 1)
	if err != nil {
		t.Fatal(err)
	}
	mp := mempool.NewMempool()
	g := gossip.NewLocalEngine(mp, cs)
	m := NewMiner("", mp, cs, g, 1, time.Second)
	if err := m.MineOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	tip, _ := cs.GetLatestBlock()
	if tip.Index != 0 {
		t.Fatal("mined empty block")
	}
	if err := g.SubmitTransaction(models.Transaction{ID: "tx", Data: "x", Timestamp: 1}); err != nil {
		t.Fatal(err)
	}
	if err := m.MineOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	chain, _ := cs.GetChain()
	if err := models.ValidateChain(chain, 1); err != nil {
		t.Fatal(err)
	}
	if len(chain) != 2 || mp.Size() != 0 {
		t.Fatal("mining failed to commit and clear pending")
	}
}
func TestSolveStopsForShutdownOrChangedTip(t *testing.T) {
	b := models.Block{Difficulty: 4}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(Solve(ctx, &b, nil), context.Canceled) {
		t.Fatal("ignored shutdown")
	}
	changed := make(chan struct{})
	close(changed)
	if !errors.Is(Solve(context.Background(), &b, changed), ErrTipChanged) {
		t.Fatal("ignored changed tip")
	}
}
