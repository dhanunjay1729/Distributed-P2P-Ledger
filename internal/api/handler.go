package api

import (
"github.com/gin-gonic/gin"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
	"p2pledger/Gossip_Engine"
)

type Handler struct {
	Store storage.Storage
	Gossip *gossip.GossipEngine   //note newly added  change to integrate 

}

//new handler functions
func NewHandler(store storage.Storage, g *gossip.GossipEngine) *Handler {
	return &Handler{
		Store:  store,
		Gossip: g,
	}
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
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	exists, err := h.Store.TransactionExists(tx.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if exists {
		c.JSON(400, gin.H{"error": "already exists"})
		return
	}

	if err := h.Store.SaveTransaction(tx); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// after saving
    h.Gossip.Gossip(tx) //newly added 

	c.JSON(200, gin.H{"status": "saved"})

}



// GET /transactions

func (h *Handler) GetTransactions(c *gin.Context) {
	// TODO:
	// 1. Load from storage
	// 2. Return JSON
	txs, err := h.Store.LoadTransactions()

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, txs)
}


// route for /gossip newdly added 

func (h *Handler) GossipReceive(c *gin.Context) {
	var tx models.Transaction

	if err := c.ShouldBindJSON(&tx); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	exists, _ := h.Store.TransactionExists(tx.ID)
	if exists {
		return
	}

	h.Store.SaveTransaction(tx)
	go h.Gossip.Gossip(tx)

	c.JSON(200, gin.H{"status": "received"})
}





















// POST /sync (optional for now)
func (h *Handler) SyncTransactions(c *gin.Context) {
	// TODO later
    

}