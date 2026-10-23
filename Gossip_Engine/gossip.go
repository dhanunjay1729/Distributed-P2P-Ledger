// Package gossip propagates messages and reconciles chain/mempool state.
package gossip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"p2pledger/internal/mempool"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
	"strings"
	"sync"
	"time"
)

const (
	defaultFanout    = 2
	maxPeers         = 32
	maxResponseBytes = storage.MaxChainBytes
)

type outgoing struct {
	peer, path string
	data       []byte
}

type GossipEngine struct {
	nextPeer   int // Protected by syncMu; rotate first peer so slow origins cannot starve others.
	peers      []string
	nodeAddr   string
	chainStore storage.ChainStorage
	mempool    *mempool.Mempool
	httpClient *http.Client
	mu         sync.Mutex // Serializes transaction, block, and reorganization decisions.
	syncMu     sync.Mutex
	changed    chan struct{}
	syncNeeded chan struct{}
	jobs       chan outgoing
}

func NewLocalEngine(mp *mempool.Mempool, cs storage.ChainStorage) *GossipEngine {
	return &GossipEngine{chainStore: cs, mempool: mp, httpClient: &http.Client{Timeout: 2 * time.Second},
		changed: make(chan struct{}), syncNeeded: make(chan struct{}, 1), jobs: make(chan outgoing, 128)}
}

func NewGossipEngine(file, addr string, mp *mempool.Mempool, cs storage.ChainStorage) (*GossipEngine, error) {
	g := NewLocalEngine(mp, cs)
	g.nodeAddr = strings.TrimRight(addr, "/")
	if file == "" {
		return g, nil
	} // Explicit single-node mode.
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read peers: %w", err)
	}
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(raw), "\n") {
		peer := strings.TrimRight(strings.TrimSpace(line), "/")
		if peer == "" || strings.HasPrefix(peer, "#") || peer == g.nodeAddr || seen[peer] {
			continue
		}
		u, err := url.Parse(peer)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("invalid peer URL %q", peer)
		}
		seen[peer] = true
		g.peers = append(g.peers, peer)
		if len(g.peers) > maxPeers {
			return nil, fmt.Errorf("at most %d peers are supported", maxPeers)
		}
	}
	return g, nil
}

func (g *GossipEngine) selectRandomPeers(n int) []string {
	if n > len(g.peers) {
		n = len(g.peers)
	}
	if n <= 0 {
		return nil
	}
	out := make([]string, n)
	for i, j := range rand.Perm(len(g.peers))[:n] {
		out[i] = g.peers[j]
	}
	return out
}

// Workers bound network concurrency; cancellation also stops retries and syncing.
func (g *GossipEngine) Run(ctx context.Context, interval time.Duration) {
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); g.worker(ctx) }()
	}
	defer workers.Wait()
	g.ResolveConflicts(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.ResolveConflicts(ctx)
		case <-g.syncNeeded:
			g.ResolveConflicts(ctx)
		}
	}
}

func (g *GossipEngine) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-g.jobs:
			for attempt := 0; attempt < 3; attempt++ {
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, job.peer+job.path, bytes.NewReader(job.data))
				if err != nil {
					break
				}
				req.Header.Set("Content-Type", "application/json")
				resp, err := g.httpClient.Do(req)
				if err == nil {
					_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
					resp.Body.Close()
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						break
					}
					// Bad data and known messages are not fixed by sending the same bytes again.
					if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
						break
					}
				}
				if attempt == 2 {
					break
				}
				timer := time.NewTimer(time.Duration(1<<attempt) * 100 * time.Millisecond)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}
	}
}

func (g *GossipEngine) broadcast(path string, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	for _, peer := range g.selectRandomPeers(defaultFanout) {
		select {
		case g.jobs <- outgoing{peer, path, data}:
		default:
		} // Periodic sync repairs dropped sends.
	}
}

func (g *GossipEngine) RequestSync() {
	select {
	case g.syncNeeded <- struct{}{}:
	default:
	}
}

func (g *GossipEngine) SubmitTransaction(tx models.Transaction) error {
	if err := models.ValidateTransaction(tx); err != nil {
		return err
	}
	g.mu.Lock()
	exists, err := g.chainStore.TransactionExists(tx.ID)
	if err == nil && exists {
		err = mempool.ErrDuplicate
	}
	if err == nil {
		err = g.mempool.TryAdd(tx)
	}
	g.mu.Unlock()
	if err == nil {
		g.broadcast("/gossip", tx)
	}
	return err
}
func (g *GossipEngine) Gossip(tx models.Transaction)         { _ = g.SubmitTransaction(tx) }
func (g *GossipEngine) HandleIncoming(tx models.Transaction) { _ = g.SubmitTransaction(tx) }

// A block is forwarded only after a successful append. A second copy fails the
// append, so no separate "seen" flag can accidentally suppress the first relay.
func (g *GossipEngine) HandleIncomingBlock(b models.Block) error {
	if err := models.ValidateBlock(b, g.chainStore.Difficulty()); err != nil {
		return err
	}
	g.mu.Lock()
	err := g.chainStore.AddBlock(b)
	if err == nil {
		ids := make([]string, 0, len(b.Transactions))
		for _, tx := range b.Transactions {
			ids = append(ids, tx.ID)
		}
		g.mempool.Remove(ids)
		close(g.changed)
		g.changed = make(chan struct{})
	}
	g.mu.Unlock()
	if err != nil {
		if errors.Is(err, storage.ErrStaleBlock) {
			g.RequestSync()
		}
		return err
	}
	g.broadcast("/block", b)
	return nil
}

func (g *GossipEngine) MiningWork() (models.Block, []models.Transaction, <-chan struct{}, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	tip, err := g.chainStore.GetLatestBlock()
	pending := g.mempool.GetPending()
	if len(pending) > models.MaxBlockTransactions {
		pending = pending[:models.MaxBlockTransactions]
	}
	return tip, pending, g.changed, err
}

func (g *GossipEngine) adopt(chain []models.Block) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	old, err := g.chainStore.GetChain()
	if err != nil {
		return err
	}
	if !models.BetterChain(chain, old) {
		return storage.ErrNotBetter
	}
	pending, err := g.mempool.PrepareReorg(old, chain)
	if err != nil {
		return err
	}
	if err := g.chainStore.ReplaceChain(chain); err != nil {
		return err
	}
	g.mempool.Replace(pending)
	close(g.changed)
	g.changed = make(chan struct{})
	return nil
}

// Each body, request duration, peer count, and overlapping sync run is bounded.
func (g *GossipEngine) get(ctx context.Context, peer, path string, out any, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, peer+path, nil)
	if err != nil {
		return err
	}
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer returned %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(raw)) > limit {
		return fmt.Errorf("peer response exceeds limit")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing response data")
	}
	return nil
}

func (g *GossipEngine) ResolveConflicts(contexts ...context.Context) {
	if !g.syncMu.TryLock() {
		return
	}
	defer g.syncMu.Unlock()
	parent := context.Background()
	if len(contexts) > 0 {
		parent = contexts[0]
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	peers := append([]string{}, g.peers...)
	if len(peers) > 0 {
		start := g.nextPeer % len(peers)
		peers = append(peers[start:], peers[:start]...)
		g.nextPeer = (start + 1) % len(peers)
	}
	for _, peer := range peers {
		if ctx.Err() != nil {
			return
		}
		var chain []models.Block
		if err := g.get(ctx, peer, "/chain", &chain, maxResponseBytes); err == nil {
			if err := models.ValidateChain(chain, g.chainStore.Difficulty()); err == nil {
				if err := g.adopt(chain); err != nil && !errors.Is(err, storage.ErrNotBetter) {
					log.Printf("sync %s: %v", peer, err)
				}
			}
		}
		var pending struct {
			Count        int                  `json:"count"`
			Transactions []models.Transaction `json:"transactions"`
		}
		if err := g.get(ctx, peer, "/mempool", &pending, 5<<20); err == nil && len(pending.Transactions) <= mempool.MaxPending {
			for _, tx := range pending.Transactions {
				_ = g.SubmitTransaction(tx)
			}
		}
	}
}

// ReadState prevents a lookup from missing a transaction as it moves between
// pending and confirmed state. Callers receive independent snapshots.
func (g *GossipEngine) ReadState() ([]models.Block, []models.Transaction, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	chain, err := g.chainStore.GetChain()
	return chain, g.mempool.GetPending(), err
}

func (g *GossipEngine) Status() (models.Block, int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	tip, err := g.chainStore.GetLatestBlock()
	return tip, g.mempool.Size(), err
}
