package gossip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"p2pledger/internal/mempool"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
	"p2pledger/internal/testutil"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func engine(t *testing.T) *GossipEngine {
	t.Helper()
	s, err := storage.NewFileChainStorage(filepath.Join(t.TempDir(), "chain.json"), 1)
	if err != nil {
		t.Fatal(err)
	}
	return NewLocalEngine(mempool.NewMempool(), s)
}
func run(t *testing.T, g *GossipEngine) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); g.Run(ctx, time.Hour) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("gossip workers did not stop")
		}
	})
}
func TestAcceptedBlockIsForwardedOnce(t *testing.T) {
	g := engine(t)
	seen := make(chan struct{}, 10)
	var calls atomic.Int32
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/block":
			calls.Add(1)
			seen <- struct{}{}
			w.WriteHeader(202)
		case "/chain":
			json.NewEncoder(w).Encode([]models.Block{models.CreateGenesisBlock(1)})
		case "/mempool":
			json.NewEncoder(w).Encode(map[string]any{"count": 0, "transactions": []models.Transaction{}})
		}
	}))
	defer peer.Close()
	g.peers = []string{peer.URL}
	run(t, g)
	tip, _ := g.chainStore.GetLatestBlock()
	b := testutil.Block(tip, "tx")
	if err := g.HandleIncomingBlock(b); err != nil {
		t.Fatal(err)
	}
	select {
	case <-seen:
	case <-time.After(time.Second):
		t.Fatal("accepted block was never forwarded")
	}
	if !errors.Is(g.HandleIncomingBlock(b), storage.ErrStaleBlock) {
		t.Fatal("duplicate accepted")
	}
	time.Sleep(30 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatalf("forwarded %d times", calls.Load())
	}
}
func TestTransactionAcceptanceIsAtomic(t *testing.T) {
	g := engine(t)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if g.SubmitTransaction(models.Transaction{ID: "same", Data: "x", Timestamp: 1}) == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted %d copies", accepted.Load())
	}
}
func TestReorgReconcilesAndCancelsOldMining(t *testing.T) {
	g := engine(t)
	genesis, _ := g.chainStore.GetLatestBlock()
	orphan := testutil.Block(genesis, "orphan")
	if err := g.HandleIncomingBlock(orphan); err != nil {
		t.Fatal(err)
	}
	g.SubmitTransaction(models.Transaction{ID: "peer", Data: "x", Timestamp: 1})
	_, _, changed, _ := g.MiningWork()
	peer1 := testutil.Block(genesis, "peer")
	peer2 := testutil.Block(peer1, "other")
	if err := g.adopt([]models.Block{genesis, peer1, peer2}); err != nil {
		t.Fatal(err)
	}
	if !g.mempool.Exists("orphan") || g.mempool.Exists("peer") {
		t.Fatal("reorg lost orphan or kept confirmed transaction")
	}
	select {
	case <-changed:
	default:
		t.Fatal("miner was not notified")
	}
	g.mempool.Remove([]string{"orphan"})
	if err := g.SubmitTransaction(orphan.Transactions[0]); err != nil {
		t.Fatal("orphan blocked by stale seen flag:", err)
	}
}
func TestDelayedPeerCannotRollbackChain(t *testing.T) {
	g := engine(t)
	genesis, _ := g.chainStore.GetLatestBlock()
	candidate := []models.Block{genesis, testutil.Block(genesis, "peer")}
	requested, release := make(chan struct{}), make(chan struct{})
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chain" {
			close(requested)
			<-release
			json.NewEncoder(w).Encode(candidate)
		} else {
			json.NewEncoder(w).Encode(map[string]any{"count": 0, "transactions": []models.Transaction{}})
		}
	}))
	defer peer.Close()
	g.peers = []string{peer.URL}
	done := make(chan struct{})
	go func() { g.ResolveConflicts(); close(done) }()
	<-requested
	b1 := testutil.Block(genesis, "local1")
	b2 := testutil.Block(b1, "local2")
	if err := g.HandleIncomingBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := g.HandleIncomingBlock(b2); err != nil {
		t.Fatal(err)
	}
	close(release)
	<-done
	tip, _ := g.chainStore.GetLatestBlock()
	if tip.Hash != b2.Hash {
		t.Fatal("stale sync replaced live progress")
	}
}
func TestInvalidPeerChainAndPendingAreIgnored(t *testing.T) {
	g := engine(t)
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chain" {
			json.NewEncoder(w).Encode([]models.Block{{Hash: "forged"}, {Hash: "forged2"}})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"count": 1, "transactions": []models.Transaction{{ID: "invalid"}}})
		}
	}))
	defer peer.Close()
	g.peers = []string{peer.URL}
	g.ResolveConflicts()
	tip, _ := g.chainStore.GetLatestBlock()
	if tip.Index != 0 || g.mempool.Size() != 0 {
		t.Fatal("accepted invalid sync data")
	}
}

func TestFullReorgNeverDropsMessages(t *testing.T) {
	g := engine(t)
	genesis, _ := g.chainStore.GetLatestBlock()
	orphan := testutil.Block(genesis, "orphan")
	if err := g.HandleIncomingBlock(orphan); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < mempool.MaxPending; i++ {
		if err := g.SubmitTransaction(models.Transaction{ID: fmt.Sprintf("pending-%d", i), Data: "x", Timestamp: 1}); err != nil {
			t.Fatal(err)
		}
	}
	b1 := testutil.Block(genesis, "peer1")
	b2 := testutil.Block(b1, "peer2")
	candidate := []models.Block{genesis, b1, b2}
	if !errors.Is(g.adopt(candidate), mempool.ErrFull) {
		t.Fatal("overflowing reorg was not refused")
	}
	tip, _ := g.chainStore.GetLatestBlock()
	if tip.Hash != orphan.Hash || g.mempool.Size() != mempool.MaxPending {
		t.Fatal("failed reorg changed state")
	}
	g.mempool.Remove([]string{"pending-0"})
	if err := g.adopt(candidate); err != nil {
		t.Fatal(err)
	}
	if !g.mempool.Exists("orphan") || g.mempool.Size() != mempool.MaxPending {
		t.Fatal("orphan was lost")
	}
}

func TestSlowFirstPeerCannotStarveOtherPeers(t *testing.T) {
	g := engine(t)
	genesis, _ := g.chainStore.GetLatestBlock()
	b := testutil.Block(genesis, "catch-up")
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chain" {
			json.NewEncoder(w).Encode([]models.Block{genesis, b})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"count": 0, "transactions": []models.Transaction{}})
		}
	}))
	defer fast.Close()
	g.peers = []string{slow.URL, fast.URL}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	g.ResolveConflicts(ctx)
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	g.ResolveConflicts(ctx)
	cancel()
	tip, _ := g.chainStore.GetLatestBlock()
	if tip.Hash != b.Hash {
		t.Fatal("slow peer starved reachable peer")
	}
}
