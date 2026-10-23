package models

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	MaxTransactionIDBytes   = 128
	MaxTransactionDataBytes = 4096
)

type Transaction struct {
	ID        string `json:"id"`
	Data      string `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

func ValidateTransaction(tx Transaction) error {
	if strings.TrimSpace(tx.ID) == "" || len(tx.ID) > MaxTransactionIDBytes || !utf8.ValidString(tx.ID) {
		return fmt.Errorf("id must contain 1 to %d bytes of valid text", MaxTransactionIDBytes)
	}
	if strings.TrimSpace(tx.Data) == "" || len(tx.Data) > MaxTransactionDataBytes || !utf8.ValidString(tx.Data) {
		return fmt.Errorf("data must contain 1 to %d bytes of valid text", MaxTransactionDataBytes)
	}
	if tx.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be positive Unix seconds")
	}
	return nil
}
