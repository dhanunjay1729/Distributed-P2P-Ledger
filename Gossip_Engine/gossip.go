/* this file contains the gossip engine implementation 
The GossipEngine struct manages the list of peers, seen transactions, and the local transaction store.
It provides methods to select random peers, check if a transaction has been seen, mark transactions as seen,
and handle incoming gossip.
The main method is Gossip, which takes a transaction, checks if it's new, marks it as seen, and forwards it to 
2 random peers.
The sendGossip method performs the actual HTTP POST to a peer, and HandleIncoming processes received transactions.
The gossipHandler is the HTTP handler for incoming gossip, and getTransactionsHandler returns all seen transactions.
The startServer method registers the handlers and starts the HTTP server.
*/

package gossip

import (
	"bytes" // The bytes package in Go provides functions for manipulating byte slices. In this code, it is used to create a new reader from the JSON data when sending gossip messages to peers. Specifically, bytes.NewReader(jsonData) creates a new reader that reads from the byte slice containing the marshaled transaction data, allowing it to be sent in the body of an HTTP POST request. For example, when the sendGossip method marshals a transaction into JSON format, it uses bytes.NewReader to create a reader for that JSON data, which is then passed to the httpClient.Post method to send the gossip message to a peer.
	"encoding/json" // The encoding/json package in Go provides functions for encoding and decoding JSON data. In this code, it is used to marshal a Transaction struct into JSON format before sending it to peers. Specifically, json.Marshal(tx) converts the Transaction struct into a JSON byte slice, which can then be sent in the body of an HTTP POST request when gossiping the transaction to other nodes in the network. For example, in the sendGossip method, the transaction is marshaled into JSON using json.Marshal, and if there is an error during this process, it logs the failure and returns without sending the gossip message.
	"fmt" 
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync" // The sync package in Go provides basic synchronization primitives such as mutexes. In this code, it is used to protect access to the 'seen' map, which keeps track of transaction IDs that have already been processed. The seenMu mutex is used to ensure that only one goroutine can read or write to the 'seen' map at a time, preventing race conditions. For example, in the isSeen method, the seenMu.RLock() is used to acquire a read lock before checking if a transaction ID is in the 'seen' map, and seenMu.RUnlock() is called afterward to release the lock. Similarly, in the acceptTransaction method, seenMu.Lock() is used to acquire a write lock when marking a transaction ID as seen, and seenMu.Unlock() is called afterward to release the lock.
	"time" 

	"p2pledger/internal/mempool"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
)

const (
	defaultFanout       = 2
	maxSendRetries      = 3 
	initialRetryBackoff = 200 * time.Millisecond 
)

// GossipEngine is the core structure that manages peer-to-peer gossip.
// It holds the list of peers, a set of seen transaction IDs (to avoid loops),
// a local store of all transactions, a mempool for unconfirmed transactions,
// and an HTTP client for outgoing requests.
type GossipEngine struct {
	peers        []string
	nodeAddr     string
	seen         map[string]bool
	seenMu       sync.RWMutex
	seenBlocks   map[string]bool
	seenBlocksMu sync.RWMutex
	httpClient   *http.Client
	store        storage.Storage
	chainStore   storage.ChainStorage
	mempool      *mempool.Mempool
	fanout       int
}

// Transaction represents a piece of data to be gossiped across the network.
// Each transaction has a unique ID, some data (e.g., "Alice pays Bob $10"), and a timestamp.


// type Transaction struct {
// 	ID        string `json:"id"`
// 	Data      string `json:"data"`
// 	Timestamp int64  `json:"timestamp"`
// }
//<--------chaged becuse alreayd defined

// NewGossipEngine creates a new gossip engine by reading peers from a file.
// Parameters:
//   - peersFile: path to a text file with one peer URL per line
//   - nodeAddr: this node's address (e.g., "http://localhost:8001") – used for logging
// Returns:
//   - *GossipEngine: initialised engine
//   - error: if file reading fails
//<---------------need to add new attribute for storage 

// The function reads the peers file, trims whitespace, and filters out empty lines and the node's own address. It also ensures that the list of peers is unique. The random number generator is seeded to ensure different random sequences each run, which is important for selecting random peers in the gossip protocol. Finally, it initializes the GossipEngine struct with the list of peers, node address, an empty seen map, an HTTP client with a timeout, the provided storage, and the mempool.
func NewGossipEngine(peersFile, nodeAddr string, store storage.Storage, mp *mempool.Mempool, cs storage.ChainStorage) (*GossipEngine, error) {
	// Read the entire peers file (fine for small files; for large lists use bufio.Scanner)
	data, err := os.ReadFile(peersFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read peers file: %w", err)
	}

	// Seed the random number generator to ensure different random sequences each run.
	rand.Seed(time.Now().UnixNano())

	// Split file content into lines and trim whitespace
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	peers := make([]string, 0, len(lines))
	seenPeers := make(map[string]struct{}, len(lines))

	for _, line := range lines {
		peer := strings.TrimSpace(line)
		if peer == "" || peer == nodeAddr {
			continue
		}
		if _, ok := seenPeers[peer]; ok {
			continue
		}
		seenPeers[peer] = struct{}{}
		peers = append(peers, peer)
	}

	return &GossipEngine{
		peers:      peers,
		nodeAddr:   nodeAddr,
		seen:       make(map[string]bool),
		seenBlocks: make(map[string]bool),
		httpClient: &http.Client{Timeout: 5 * time.Second},
		store:      store,
		chainStore: cs,
		mempool:    mp,
		fanout:     defaultFanout,
	}, nil
}

// selectRandomPeers picks n distinct random peers from the peer list.
// If n >= number of peers, it returns a shuffled copy of all peers.
// This is used to implement the gossip fan‑out (k=2).
func (g *GossipEngine) selectRandomPeers(n int) []string {
	if n <= 0 || len(g.peers) == 0 {
		return nil
	}
	if n >= len(g.peers) {
		// Return a shuffled copy of all peers
		shuffled := make([]string, len(g.peers))
		copy(shuffled, g.peers)
		rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		return shuffled
	}
	// Pick n distinct random indices using rand.Perm
	indices := rand.Perm(len(g.peers))[:n]
	selected := make([]string, n)
	for i, idx := range indices {
		selected[i] = g.peers[idx]
	}
	return selected
}

// isSeen checks whether a transaction ID has already been processed. 
// It uses a read lock to safely access the 'seen' map, allowing multiple concurrent reads while preventing writes during the check. This method is called before accepting a transaction to determine if it should be processed or ignored.
func (g *GossipEngine) isSeen(txID string) bool {
	g.seenMu.RLock()
	defer g.seenMu.RUnlock()
	return g.seen[txID]
}

// NOTE:
// Removed legacy markSeen() that referenced g.txMu / g.transactions.
// Dedup + persistence is now handled by acceptTransaction() using:
// 1) in-memory seen map
// 2) persistent store.TransactionExists + store.SaveTransaction

// acceptTransaction checks if a transaction is new and accepts it if so.
// It returns true if the transaction was accepted, false if it was already known,
// and an error if there was a problem during the process.
//
// The method first checks the in-memory 'seen' map to quickly determine if the
// transaction ID has already been processed. If it has, it returns false without
// further processing. If it's new, it marks the transaction ID as seen in the
// 'seen' map. Then it checks the persistent store to see if the transaction
// already exists (e.g., already mined into a block). If it exists, it returns
// false. If it's truly new, it adds the transaction to the mempool (if available)
// or falls back to direct storage.
func (g *GossipEngine) acceptTransaction(tx models.Transaction) (bool, error) {
	g.seenMu.Lock()
	if g.seen[tx.ID] {
		g.seenMu.Unlock()
		return false, nil
	}
	g.seen[tx.ID] = true
	g.seenMu.Unlock()

	// Check permanent storage (for already-mined transactions)
	exists, err := g.store.TransactionExists(tx.ID)
	if err != nil {
		g.seenMu.Lock()
		delete(g.seen, tx.ID)
		g.seenMu.Unlock()
		return false, fmt.Errorf("exists check failed: %w", err)
	}
	if exists {
		return false, nil
	}

	// If mempool is available, add there (transactions wait for mining).
	// Otherwise fall back to direct storage (legacy/test mode).
	if g.mempool != nil {
		if !g.mempool.Add(tx) {
			return false, nil // already in mempool
		}
	} else {
		if err := g.store.SaveTransaction(tx); err != nil {
			g.seenMu.Lock()
			delete(g.seen, tx.ID)
			g.seenMu.Unlock()
			return false, fmt.Errorf("save failed: %w", err)
		}
	}

	return true, nil
}

// Gossip is the main method that starts the gossip propagation.
// It marks the transaction as seen (if new) and forwards it to 2 random peers.
// This method is called either by the /Transaction endpoint (user‑initiated)
// or by HandleIncoming when a transaction is received from another node.
func (g *GossipEngine) Gossip(tx models.Transaction) {
	accepted, err := g.acceptTransaction(tx)
	if err != nil {
		log.Printf("[%s] tx %s rejected due to error: %v", g.nodeAddr, tx.ID, err)
		return
	}
	if !accepted {
		log.Printf("[%s] tx %s already known; skipping", g.nodeAddr, tx.ID)
		return
	}

	log.Printf("[%s] Gossiping tx %s: %s", g.nodeAddr, tx.ID, tx.Data)

	for _, peerURL := range g.selectRandomPeers(g.fanout) {
		go g.sendGossip(peerURL, tx)
	}
}

// sendGossip performs the actual HTTP POST request to a single peer.
// It is called asynchronously by Gossip.
func (g *GossipEngine) sendGossip(peerURL string, tx models.Transaction) {
	jsonData, err := json.Marshal(tx)
	if err != nil {
		log.Printf("[%s] Failed to marshal tx %s: %v", g.nodeAddr, tx.ID, err)
		return
	}

	backoff := initialRetryBackoff
	for attempt := 1; attempt <= maxSendRetries; attempt++ {
		resp, err := g.httpClient.Post(peerURL+"/gossip", "application/json", bytes.NewReader(jsonData))
		if err == nil {
			status := resp.StatusCode
			resp.Body.Close()
			if status == http.StatusOK {
				return
			}
			err = fmt.Errorf("status %d", status)
		}

		if attempt == maxSendRetries {
			log.Printf("[%s] Failed to send tx %s to %s after %d attempts: %v", g.nodeAddr, tx.ID, peerURL, attempt, err)
			return
		}

		log.Printf("[%s] Retry %d/%d for tx %s to %s: %v", g.nodeAddr, attempt, maxSendRetries, tx.ID, peerURL, err)
		time.Sleep(backoff)
		backoff *= 2
	}
}

// HandleIncoming processes a transaction received via /gossip.
// If the transaction is new, it marks it as seen and then forwards it further (gossip).
// This implements the "if exists → ignore; else → forward" rule.
func (g *GossipEngine) HandleIncoming(tx models.Transaction) {
	// Single path for local+incoming: accept + persist + forward if new.
	g.Gossip(tx)
}

// NOTE:
// Removed legacy net/http handlers from this package.

// --- Block Gossip ---

// GossipBlock propagates a newly mined or newly received block to peers.
func (g *GossipEngine) GossipBlock(block models.Block) {
	g.seenBlocksMu.Lock()
	if g.seenBlocks[block.Hash] {
		g.seenBlocksMu.Unlock()
		return // already gossiped
	}
	g.seenBlocks[block.Hash] = true
	g.seenBlocksMu.Unlock()

	targets := g.selectRandomPeers(g.fanout)
	log.Printf("[%s] 📢 Gossiping Block #%d to %d peers: %v", g.nodeAddr, block.Index, len(targets), targets)

	for _, peer := range targets {
		go g.sendBlockGossip(peer, block)
	}
}

// sendBlockGossip performs the HTTP POST to a peer for a block.
func (g *GossipEngine) sendBlockGossip(peerURL string, block models.Block) {
	jsonData, err := json.Marshal(block)
	if err != nil {
		log.Printf("[%s] ❌ Failed to marshal block: %v", g.nodeAddr, err)
		return
	}

	url := fmt.Sprintf("%s/block", peerURL)
	backoff := initialRetryBackoff

	for i := 0; i < maxSendRetries; i++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := g.httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusConflict {
				return // success or already known
			}
		}

		time.Sleep(backoff)
		backoff *= 2
	}
}

// HandleIncomingBlock processes a block received from a peer.
// It verifies if the block is new, attempts to append it to the local chain,
// and if successful, removes the transactions from the mempool and gossips it further.
func (g *GossipEngine) HandleIncomingBlock(block models.Block) error {
	g.seenBlocksMu.Lock()
	if g.seenBlocks[block.Hash] {
		g.seenBlocksMu.Unlock()
		return fmt.Errorf("block already seen")
	}
	g.seenBlocksMu.Unlock()

	// Try to add the block to the local chain
	if g.chainStore != nil {
		if err := g.chainStore.AddBlock(block); err != nil {
			// This can happen if the block is invalid, or if we have a conflict
			// (i.e. another miner won the race and our chain is on a different path).
			// Let's trigger a network-wide consensus check.
			go g.ResolveConflicts()
			return fmt.Errorf("failed to add block, triggering conflict resolution: %w", err)
		}
	}

	// Block accepted! Mark as seen.
	g.seenBlocksMu.Lock()
	g.seenBlocks[block.Hash] = true
	g.seenBlocksMu.Unlock()

	log.Printf("[%s] 📥 Accepted Block #%d from network (Hash: %s)", g.nodeAddr, block.Index, block.Hash[:10]+"...")

	// Remove these transactions from the waiting room, since they are now confirmed
	if g.mempool != nil && len(block.Transactions) > 0 {
		txIDs := make([]string, len(block.Transactions))
		for i, tx := range block.Transactions {
			txIDs[i] = tx.ID
		}
		g.mempool.Remove(txIDs)
	}

	// Propagate it further
	go g.GossipBlock(block)

	return nil
}

// ResolveConflicts implements the Longest Chain Rule.
// It asks all peers for their chains, and if it finds a valid chain that is
// longer than its own, it replaces its local chain with the peer's chain.
func (g *GossipEngine) ResolveConflicts() {
	if g.chainStore == nil {
		return
	}
	
	log.Printf("[%s] 🔄 Resolving conflicts... checking peers for longer chains", g.nodeAddr)
	
	localChain, err := g.chainStore.GetChain()
	if err != nil {
		return
	}
	
	maxLength := len(localChain)
	var longestChain []models.Block
	chainReplaced := false

	for _, peer := range g.peers {
		url := fmt.Sprintf("%s/chain", peer)
		resp, err := g.httpClient.Get(url)
		if err != nil {
			continue // peer might be down
		}

		var peerChain []models.Block
		if err := json.NewDecoder(resp.Body).Decode(&peerChain); err == nil {
			// Check if peer's chain is longer AND cryptographically valid
			if len(peerChain) > maxLength && storage.ValidateChain(peerChain) {
				maxLength = len(peerChain)
				longestChain = peerChain
				chainReplaced = true
			}
		}
		resp.Body.Close()
	}

	// If we found a longer valid chain, adopt it
	if chainReplaced {
		log.Printf("[%s] ⚠️ Fork resolved! Replaced local chain with longer chain from network (New length: %d)", g.nodeAddr, maxLength)
		if err := g.chainStore.ReplaceChain(longestChain); err != nil {
			log.Printf("[%s] ❌ Failed to replace chain during resolution: %v", g.nodeAddr, err)
		}
	} else {
		log.Printf("[%s] ✅ Local chain is the longest (or tied). No replacement needed.", g.nodeAddr)
	}
}
