// Package testutil builds valid, low-difficulty blocks for regression tests.
package testutil

import (
	"p2pledger/internal/models"
	"strings"
	"time"
)

func Block(prev models.Block, ids ...string) models.Block {
	b := models.Block{Version: models.ProtocolVersion, Difficulty: prev.Difficulty, Index: prev.Index + 1,
		Timestamp: max(time.Now().Unix(), prev.Timestamp), PrevHash: prev.Hash, Transactions: []models.Transaction{}}
	for _, id := range ids {
		b.Transactions = append(b.Transactions, models.Transaction{ID: id, Data: "message " + id, Timestamp: 1})
	}
	for {
		b.Hash = b.CalculateHash()
		if strings.HasPrefix(b.Hash, strings.Repeat("0", b.Difficulty)) {
			return b
		}
		b.Nonce++
	}
}
