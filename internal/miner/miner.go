// Package miner solves small proof-of-work puzzles only when messages are pending.
package miner

import (
	"context"
	"errors"
	"log"
	gossip "p2pledger/Gossip_Engine"
	"p2pledger/internal/mempool"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
	"runtime"
	"strings"
	"time"
)

var ErrTipChanged = errors.New("chain changed while mining")

type Miner struct {
	engine     *gossip.GossipEngine
	difficulty int
	interval   time.Duration
}

func NewMiner(_ string, mp *mempool.Mempool, cs storage.ChainStorage, g *gossip.GossipEngine, d int, interval time.Duration) *Miner {
	if g == nil {
		g = gossip.NewLocalEngine(mp, cs)
	}
	return &Miner{engine: g, difficulty: d, interval: interval}
}

func Solve(ctx context.Context, b *models.Block, changed <-chan struct{}) error {
	if err := models.CheckDifficulty(b.Difficulty); err != nil {
		return err
	}
	target := strings.Repeat("0", b.Difficulty)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
			return ErrTipChanged
		default:
		}
		b.Hash = b.CalculateHash()
		if strings.HasPrefix(b.Hash, target) {
			return nil
		}
		if b.Nonce == ^uint64(0) {
			return errors.New("nonce space exhausted")
		}
		b.Nonce++
		if b.Nonce%1024 == 0 {
			runtime.Gosched()
		}
	}
}

func (m *Miner) MineOnce(ctx context.Context) error {
	tip, pending, changed, err := m.engine.MiningWork()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	b := models.Block{Version: models.ProtocolVersion, Difficulty: m.difficulty, Index: tip.Index + 1,
		Timestamp: max(time.Now().Unix(), tip.Timestamp), Transactions: pending, PrevHash: tip.Hash}
	if err := Solve(ctx, &b, changed); err != nil {
		return err
	}
	return m.engine.HandleIncomingBlock(b)
}

func (m *Miner) Start(contexts ...context.Context) {
	ctx := context.Background()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	if err := models.CheckDifficulty(m.difficulty); err != nil || m.interval <= 0 {
		log.Printf("invalid miner configuration")
		return
	}
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.MineOnce(ctx); err != nil && !errors.Is(err, ErrTipChanged) && !errors.Is(err, storage.ErrStaleBlock) && !errors.Is(err, context.Canceled) {
				log.Printf("mining: %v", err)
			}
		}
	}
}
