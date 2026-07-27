/*
Package miner implements the Proof of Work (PoW) consensus mechanism.

The Miner runs as a background process. At a configurable interval, it checks
the Mempool for unconfirmed transactions. If it finds any, it packs them into
a new Block and begins the Proof of Work puzzle (Hashcash).

The puzzle requires the miner to repeatedly guess a 'Nonce' until the resulting
SHA-256 hash of the block starts with a specific number of zeros (the Difficulty).

Once the puzzle is solved, the block is added to the permanent ChainStorage,
and the transactions are removed from the Mempool.
*/
package miner

import (
	"log"
	"strings"
	"time"

	"p2pledger/internal/mempool"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
)

// Miner handles the creation of new blocks via Proof of Work.
type Miner struct {
	nodeAddr   string
	mempool    *mempool.Mempool
	chainStore storage.ChainStorage
	difficulty int           // Number of leading zeros required in the hash
	interval   time.Duration // How often to attempt mining
}

// NewMiner creates a new Miner instance.
func NewMiner(nodeAddr string, mp *mempool.Mempool, cs storage.ChainStorage, diff int, interval time.Duration) *Miner {
	return &Miner{
		nodeAddr:   nodeAddr,
		mempool:    mp,
		chainStore: cs,
		difficulty: diff,
		interval:   interval,
	}
}

// Start runs the mining loop in the background. It blocks forever, so it
// should be called as a goroutine.
func (m *Miner) Start() {
	log.Printf("[%s] ⛏️  Miner started. Difficulty: %d, Interval: %v", m.nodeAddr, m.difficulty, m.interval)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		<-ticker.C
		m.mineBlock()
	}
}

// mineBlock performs one mining cycle.
func (m *Miner) mineBlock() {
	// 1. Check if there is anything to mine
	pendingTxs := m.mempool.GetPending()
	if len(pendingTxs) == 0 {
		return // Nothing to do
	}

	// 2. Get the latest block to link to
	latestBlock, err := m.chainStore.GetLatestBlock()
	if err != nil {
		log.Printf("[%s] Miner error getting latest block: %v", m.nodeAddr, err)
		return
	}

	// 3. Create the blueprint for the new block
	newBlock := models.Block{
		Index:        latestBlock.Index + 1,
		Timestamp:    time.Now().Unix(),
		Transactions: pendingTxs,
		PrevHash:     latestBlock.Hash,
		Nonce:        0, // Start guessing at 0
	}

	log.Printf("[%s] ⛏️  Mining block #%d with %d transactions...", m.nodeAddr, newBlock.Index, len(pendingTxs))
	start := time.Now()

	// 4. The Proof of Work Loop (The "Work")
	targetPrefix := strings.Repeat("0", m.difficulty)
	for {
		hash := newBlock.CalculateHash()
		
		// Did we win?
		if strings.HasPrefix(hash, targetPrefix) {
			newBlock.Hash = hash
			break
		}
		
		// Wrong guess, increment Nonce and try again
		newBlock.Nonce++
	}

	elapsed := time.Since(start)
	log.Printf("[%s] ✅ Block #%d mined! Hash: %s (Nonce: %d, took %v)", 
		m.nodeAddr, newBlock.Index, newBlock.Hash[:10]+"...", newBlock.Nonce, elapsed)

	// 5. Save the block to the permanent chain
	if err := m.chainStore.AddBlock(newBlock); err != nil {
		log.Printf("[%s] ❌ Failed to save mined block: %v", m.nodeAddr, err)
		return
	}

	// 6. Remove the mined transactions from the waiting room
	txIDs := make([]string, len(pendingTxs))
	for i, tx := range pendingTxs {
		txIDs[i] = tx.ID
	}
	m.mempool.Remove(txIDs)

	// TODO: Part 5 - Gossip this new block to peers!
}
