// Package api exposes a bounded JSON API for the local ledger.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	gossip "p2pledger/Gossip_Engine"
	"p2pledger/internal/mempool"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
	"unicode/utf8"
)

const MaxRequestBytes = 1 << 20

type Handler struct {
	ChainStore storage.ChainStorage
	Gossip     *gossip.GossipEngine
	Mempool    *mempool.Mempool
}

func NewHandler(cs storage.ChainStorage, mp *mempool.Mempool, engines ...*gossip.GossipEngine) *Handler {
	var g *gossip.GossipEngine
	if len(engines) > 0 {
		g = engines[0]
	}
	if g == nil {
		g = gossip.NewLocalEngine(mp, cs)
	}
	return &Handler{ChainStore: cs, Mempool: mp, Gossip: g}
}

func NewRouter(h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)
	r.POST("/transaction", h.AddTransaction)
	r.GET("/transactions", h.GetTransactions)
	r.GET("/transaction/:id", h.GetTransaction)
	r.POST("/gossip", h.GossipReceive)
	r.POST("/block", h.ReceiveBlock)
	r.GET("/chain", h.GetChain)
	r.GET("/mempool", h.GetMempool)
	r.GET("/healthz", h.GetHealthz)
	r.GET("/status", h.GetStatus)
	return r
}

func decode(c *gin.Context, out any) bool {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "Content-Type must be application/json"})
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxRequestBytes)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body exceeds 1 MiB"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read request body"})
		}
		return false
	}
	if !utf8.Valid(raw) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON must contain valid UTF-8"})
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected exactly one JSON value"})
		return false
	}
	return true
}

func (h *Handler) accept(c *gin.Context, incoming bool) {
	var tx models.Transaction
	if !decode(c, &tx) {
		return
	}
	if err := models.ValidateTransaction(tx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Gossip.SubmitTransaction(tx); err != nil {
		switch {
		case errors.Is(err, mempool.ErrDuplicate):
			if incoming {
				c.JSON(http.StatusOK, gin.H{"status": "already_known"})
			} else {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			}
		case errors.Is(err, mempool.ErrFull):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction could not be accepted"})
		}
		return
	}
	status := http.StatusAccepted
	if incoming {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"status": "pending", "id": tx.ID})
}
func (h *Handler) AddTransaction(c *gin.Context) { h.accept(c, false) }
func (h *Handler) GossipReceive(c *gin.Context)  { h.accept(c, true) }

func (h *Handler) ReceiveBlock(c *gin.Context) {
	var b models.Block
	if !decode(c, &b) {
		return
	}
	if err := models.ValidateBlock(b, h.ChainStore.Difficulty()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Gossip.HandleIncomingBlock(b); err != nil {
		code := http.StatusUnprocessableEntity
		if errors.Is(err, storage.ErrStaleBlock) {
			code = http.StatusConflict
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "block_accepted"})
}
func (h *Handler) GetChain(c *gin.Context) {
	chain, err := h.ChainStore.GetChain()
	if err != nil {
		c.JSON(500, gin.H{"error": "chain unavailable"})
		return
	}
	c.JSON(http.StatusOK, chain)
}
func (h *Handler) GetTransactions(c *gin.Context) {
	chain, err := h.ChainStore.GetChain()
	if err != nil {
		c.JSON(500, gin.H{"error": "chain unavailable"})
		return
	}
	txs := []models.Transaction{}
	for _, b := range chain {
		txs = append(txs, b.Transactions...)
	}
	c.JSON(http.StatusOK, txs)
}
func (h *Handler) GetTransaction(c *gin.Context) {
	id := c.Param("id")
	chain, pending, err := h.Gossip.ReadState()
	if err != nil {
		c.JSON(500, gin.H{"error": "chain unavailable"})
		return
	}
	for _, b := range chain {
		for _, tx := range b.Transactions {
			if tx.ID == id {
				c.JSON(200, gin.H{"status": "confirmed", "transaction": tx, "block_index": b.Index, "confirmations": len(chain) - b.Index})
				return
			}
		}
	}
	for _, tx := range pending {
		if tx.ID == id {
			c.JSON(200, gin.H{"status": "pending", "transaction": tx, "confirmations": 0})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("transaction %q not found", id)})
}
func (h *Handler) GetMempool(c *gin.Context) {
	txs := h.Mempool.GetPending()
	c.JSON(http.StatusOK, gin.H{"count": len(txs), "transactions": txs})
}
func (h *Handler) GetStatus(c *gin.Context) {
	b, pending, err := h.Gossip.Status()
	if err != nil {
		c.JSON(503, gin.H{"error": "chain unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": models.ProtocolVersion, "difficulty": h.ChainStore.Difficulty(), "height": b.Index, "hash": b.Hash, "pending": pending})
}
func (h *Handler) GetHealthz(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) }
