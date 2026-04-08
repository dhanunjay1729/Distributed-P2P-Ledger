package gossip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"p2pledger/internal/storage"
	"p2pledger/internal/models"

)

// Transaction represents a piece of data to be gossiped across the network.
// Each transaction has a unique ID, some data (e.g., "Alice pays Bob $10"), and a timestamp.


// type Transaction struct {
// 	ID        string `json:"id"`
// 	Data      string `json:"data"`
// 	Timestamp int64  `json:"timestamp"`
// }
//<--------chaged becuse alreayd defined




// GossipEngine is the core structure that manages peer-to-peer gossip.
// It holds the list of peers, a set of seen transaction IDs (to avoid loops),
// a local store of all transactions, and an HTTP client for outgoing requests.
type GossipEngine struct {
	peers        []string          // List of peer URLs (e.g., "http://localhost:8002")
	nodeAddr     string            // This node's own address (used only for logging)
	seen         map[string]bool   // Tracks already processed transaction IDs
	seenMu       sync.RWMutex      // Mutex for seen map (concurrent read/write)
	transactions []models.Transaction     // All transactions this node has ever seen (for GET /Transactions)
	txMu         sync.RWMutex      // Mutex for transactions slice
	httpClient   *http.Client      // HTTP client with timeout
	store storage.Storage         //<-----------------creating a storgae attribute 
}




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

	// Split file content into lines and trim whitespace
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	peers := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		peers = append(peers, line)
	}

	return &GossipEngine{
		peers:        peers,
		nodeAddr:     nodeAddr,
		seen:         make(map[string]bool),
		store:        store,//adding the new store thingy
		transactions: []models.Transaction{},
		httpClient:   &http.Client{Timeout: 5 * time.Second},
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



// markSeen records a transaction as seen and appends it to the local transactions list.
// It acquires both write locks (seen and transactions) to ensure consistency.
func (g *GossipEngine) markSeen(tx models.Transaction) {
	g.seenMu.Lock()
	defer g.seenMu.Unlock()
	g.txMu.Lock()
	defer g.txMu.Unlock()
	g.seen[tx.ID] = true
	g.store.SaveTransaction(tx)//<------------------------------newly added
	g.transactions = append(g.transactions, tx)
}

// Gossip is the main method that starts the gossip propagation.
// It marks the transaction as seen (if new) and forwards it to 2 random peers.
// This method is called either by the /Transaction endpoint (user‑initiated)
// or by HandleIncoming when a transaction is received from another node.
func (g *GossipEngine) Gossip(tx models.Transaction) {
	if g.isSeen(tx.ID) {
		log.Printf("[%s] Gossip called but tx %s already seen – ignoring", g.nodeAddr, tx.ID)
		return
	}
	g.markSeen(tx)
	log.Printf("[%s] Gossiping tx %s: %s", g.nodeAddr, tx.ID, tx.Data)

	peersToSend := g.selectRandomPeers(2)
	for _, peerURL := range peersToSend {
		// Launch a goroutine for each peer to avoid blocking
		go g.sendGossip(peerURL, tx)
	}
}

// sendGossip performs the actual HTTP POST request to a single peer.
// It is called asynchronously by Gossip.
func (g *GossipEngine) sendGossip(peerURL string, tx models.Transaction) {
	jsonData, err := json.Marshal(tx)
	if err != nil {
		log.Printf("[%s] Failed to marshal tx: %v", g.nodeAddr, err)
		return
	}
	resp, err := g.httpClient.Post(peerURL+"/gossip", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Printf("[%s] Failed to send gossip to %s: %v", g.nodeAddr, peerURL, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] Peer %s returned status %d", g.nodeAddr, peerURL, resp.StatusCode)
	}
}

// HandleIncoming processes a transaction received via the /gossip endpoint.
// If the transaction is new, it marks it as seen and then forwards it further (gossip).
// This implements the "if exists → ignore; else → forward" rule.
func (g *GossipEngine) HandleIncoming(tx models.Transaction) {
	if g.isSeen(tx.ID) {
		log.Printf("[%s] Ignoring duplicate tx %s", g.nodeAddr, tx.ID)
		return
	}
	g.markSeen(tx)
	log.Printf("[%s] Received new tx %s: %s (timestamp %d)", g.nodeAddr, tx.ID, tx.Data, tx.Timestamp)

	// Forward to other peers asynchronously
	go g.Gossip(tx)
}









// i am skippinig these for now ;like manking changes in mine 


// gossipHandler is the HTTP handler for POST /gossip.
// It reads the transaction JSON, then calls HandleIncoming in a goroutine.



func (g *GossipEngine) gossipHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	var tx models.Transaction
	if err := json.Unmarshal(body, &tx); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	go g.HandleIncoming(tx)
	w.WriteHeader(http.StatusOK)
}



//commetsn by me: call bakc function for gossip how to move this to /internals/api?
 




// getTransactionsHandler is the HTTP handler for GET /Transactions.
// It returns all transactions this node has seen as a JSON array.
func (g *GossipEngine) getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
		return
	}
	g.txMu.RLock()
	defer g.txMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g.transactions)
}

//coomments by sai:conciflcts wwiht mt /transaction ,should i edit in /internals/api?













// startServer registers the HTTP handlers and starts listening.
func (g *GossipEngine) startServer(port string) {
	http.HandleFunc("/gossip", g.gossipHandler)
	http.HandleFunc("/Transactions", g.getTransactionsHandler)
	addr := ":" + port
	log.Printf("[%s] Gossip engine listening on %s", g.nodeAddr, addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("[%s] Server failed: %v", g.nodeAddr, err)
	}
}

// func main() {
// 	// Command-line arguments: node-address, port, [peers-file]
// 	if len(os.Args) < 3 {
// 		fmt.Println("Usage: go run main.go <node-address> <port> [peers-file]")
// 		fmt.Println("Example: go run main.go http://localhost:8001 8001 peers.txt")
// 		os.Exit(1)
// 	}
// 	nodeAddr := os.Args[1]
// 	port := os.Args[2]
// 	peersFile := "peers.txt"
// 	if len(os.Args) >= 4 {
// 		peersFile = os.Args[3]
// 	}

// 	rand.Seed(time.Now().UnixNano()) // Seed random number generator

// 	engine, err := NewGossipEngine(peersFile, nodeAddr)
// 	if err != nil {
// 		log.Fatalf("Failed to create gossip engine: %v", err)
// 	}

// 	// Start the gossip HTTP server in a background goroutine
// 	go engine.startServer(port)

// 	// POST /Transaction endpoint – allows users to submit a new transaction
// 	http.HandleFunc("/Transaction", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method != http.MethodPost {
// 			http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
// 			return
// 		}
// 		var req struct {
// 			Data string `json:"data"`
// 		}
// 		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 			http.Error(w, "Invalid JSON", http.StatusBadRequest)
// 			return
// 		}
// 		tx := Transaction{
// 			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
// 			Data:      req.Data,
// 			Timestamp: time.Now().Unix(),
// 		}
// 		engine.Gossip(tx) // start the gossip chain
// 		w.WriteHeader(http.StatusAccepted)
// 		fmt.Fprintf(w, "Transaction gossiped with ID %s\n", tx.ID)
// 	})

// 	log.Printf("[%s] Ready. Endpoints: POST /Transaction, POST /gossip, GET /Transactions", nodeAddr)

// 	// Block forever (keep the main goroutine alive)
// 	select {}
// }

