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
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

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
// a local store of all transactions, and an HTTP client for outgoing requests.
type GossipEngine struct {
	peers      []string
	nodeAddr   string
	seen       map[string]bool
	seenMu     sync.RWMutex
	httpClient *http.Client
	store      storage.Storage
	fanout     int
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
func NewGossipEngine(peersFile, nodeAddr string, store storage.Storage) (*GossipEngine, error) {
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
		httpClient: &http.Client{Timeout: 5 * time.Second},
		store:      store,
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
func (g *GossipEngine) acceptTransaction(tx models.Transaction) (bool, error) {
	g.seenMu.Lock()
	if g.seen[tx.ID] {
		g.seenMu.Unlock()
		return false, nil
	}
	g.seen[tx.ID] = true
	g.seenMu.Unlock()

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

	if err := g.store.SaveTransaction(tx); err != nil {
		g.seenMu.Lock()
		delete(g.seen, tx.ID)
		g.seenMu.Unlock()
		return false, fmt.Errorf("save failed: %w", err)
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
func (g *GossipEngine) HandleIncoming(tx models.Transaction) {
	// Single path for local+incoming: accept + persist + forward if new.
	g.Gossip(tx)
}

// NOTE:
// Removed legacy net/http handlers from this package:
// - gossipHandlerGreat — here is the final summary of what was changed in gossip.go, in simple words.

