/*
This file contains the API handlers for the P2P ledger application. 
API handler is a function that processes incoming HTTP requests,
interacts with the storage layer to manage transactions, and returns appropriate responses.
It defines a Handler struct that has a reference to the storage layer and 
the gossip engine. 
The Handler struct has methods to handle incoming HTTP requests
for adding transactions, getting transactions, and receiving gossip messages. 
The AddTransaction method processes incoming transaction data, checks if it 
already exists, saves it to storage, and then gossips it to peers. 
The GetTransactions method retrieves all transactions from storage and 
returns them as JSON. 
The GossipReceive method handles incoming gossip messages, checks if the
transaction is new, saves it, and gossips it further if necessary.

*/
package api

import (
	"net/http" 
	"github.com/gin-gonic/gin" // for HTTP routing and handling
	gossip "p2pledger/Gossip_Engine"
	"p2pledger/internal/mempool"
	"p2pledger/internal/models" 
	"p2pledger/internal/storage"
)

type Handler struct {
	ChainStore storage.ChainStorage // For reading the blockchain
	Gossip     *gossip.GossipEngine
	Mempool    *mempool.Mempool
}

// NewHandler creates an API handler.
func NewHandler(cs storage.ChainStorage, mp *mempool.Mempool, g ...*gossip.GossipEngine) *Handler {
	var ge *gossip.GossipEngine
	if len(g) > 0 {
		ge = g[0]
	}
	return &Handler{
		ChainStore: cs,
		Gossip:     ge,
		Mempool:    mp,
	}
}

// isValidTransaction checks if the transaction has all required fields (ID, Data, Timestamp) and that they are valid (non-empty ID and Data, positive Timestamp).
func isValidTransaction(tx models.Transaction) bool {
	if tx.ID == "" || tx.Data == "" || tx.Timestamp <= 0 {
		return false
	}
	return true
}

// POST /transaction
func (h *Handler) AddTransaction(c *gin.Context) {
	var tx models.Transaction
     // TODO:
	// 1. Bind JSON
	// 2. Check exists
	// 3. Save
	// 4. Return response
	if err := c.ShouldBindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !isValidTransaction(tx) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id, data, timestamp are required"})
		return
	}

	// Dedup check: is it already in the mempool?
	if h.Mempool != nil && h.Mempool.Exists(tx.ID) {
		c.JSON(http.StatusConflict, gin.H{"error": "already exists"})
		return
	}

	// Dedup check: is it already in permanent storage (already mined)?
	if h.ChainStore != nil {
		exists, err := h.ChainStore.TransactionExists(tx.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "already exists"})
			return
		}
	}

	// If gossip engine is wired, use gossip path (adds to mempool + forwards to peers).
	if h.Gossip != nil {
		h.Gossip.Gossip(tx)
		c.JSON(http.StatusAccepted, gin.H{"status": "gossip_started"})
		return
	}

	// Fallback: add directly to mempool (single-node / no-gossip mode).
	if h.Mempool != nil {
		h.Mempool.Add(tx)
		c.JSON(http.StatusAccepted, gin.H{"status": "added_to_mempool"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "no mempool or gossip engine configured"})
}

// GET /transactions
// This is a legacy endpoint, now modified to extract all transactions from the entire blockchain.
func (h *Handler) GetTransactions(c *gin.Context) {
	if h.ChainStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "chain storage not configured"})
		return
	}

	chain, err := h.ChainStore.GetChain()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var allTxs []models.Transaction
	for _, block := range chain {
		allTxs = append(allTxs, block.Transactions...)
	}

	c.JSON(http.StatusOK, allTxs)
}


// route for /gossip newdly added 

func (h *Handler) GossipReceive(c *gin.Context) {
	var tx models.Transaction

	if err := c.ShouldBindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !isValidTransaction(tx) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id, data, timestamp are required"})
		return
	}

	if h.Gossip == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "gossip engine not configured"})
		return
	}

	h.Gossip.HandleIncoming(tx)
	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

// POST /sync (optional for now)
func (h *Handler) SyncTransactions(c *gin.Context) {
	// TODO later


}

// GET /mempool — returns all unconfirmed transactions waiting to be mined.
func (h *Handler) GetMempool(c *gin.Context) {
	if h.Mempool == nil {
		c.JSON(http.StatusOK, []models.Transaction{})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"count":        h.Mempool.Size(),
		"transactions": h.Mempool.GetPending(),
	})
}

// POST /block — receives a new block mined by a peer.
func (h *Handler) ReceiveBlock(c *gin.Context) {
	var block models.Block
	if err := c.ShouldBindJSON(&block); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Basic check
	if block.Hash == "" || block.PrevHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid block format"})
		return
	}

	// Hand off to gossip engine which will validate and store it
	if h.Gossip != nil {
		if err := h.Gossip.HandleIncomingBlock(block); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"status": "block_accepted"})
		return
	}

	c.JSON(http.StatusNotImplemented, gin.H{"error": "gossip engine not configured"})
}

// GET /chain — returns the full blockchain. Used by peers to resolve conflicts.
func (h *Handler) GetChain(c *gin.Context) {
	if h.ChainStore == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "chain storage not configured"})
		return
	}
	
	chain, err := h.ChainStore.GetChain()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, chain)
}

// GET /healthz — liveness probe for Docker/K8s.
func (h *Handler) GetHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}