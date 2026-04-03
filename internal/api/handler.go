package api

import (
	"github.com/gin-gonic/gin"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
)

type Handler struct {
	Store storage.Storage
}

func NewHandler(store storage.Storage) *Handler {
	return &Handler{Store: store}
}

// POST /transaction
func (h *Handler) AddTransaction(c *gin.Context) {
	var tx models.Transaction

	// TODO:
	// 1. Bind JSON
	// 2. Check exists
	// 3. Save
	// 4. Return response
}

// GET /transactions
func (h *Handler) GetTransactions(c *gin.Context) {
	// TODO:
	// 1. Load from storage
	// 2. Return JSON
}

// POST /sync (optional for now)
func (h *Handler) SyncTransactions(c *gin.Context) {
	// TODO later
}