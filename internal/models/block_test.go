package models_test

import (
	"encoding/json"
	"os"
	"p2pledger/internal/models"
	"p2pledger/internal/testutil"
	"strings"
	"testing"
)

func TestHashProtectsAllFields(t *testing.T) {
	original := testutil.Block(models.CreateGenesisBlock(1), "id")
	changes := map[string]func(*models.Block){
		"data":                  func(b *models.Block) { b.Transactions[0].Data = "altered" },
		"transaction timestamp": func(b *models.Block) { b.Transactions[0].Timestamp++ },
		"index":                 func(b *models.Block) { b.Index++ },
		"timestamp":             func(b *models.Block) { b.Timestamp++ },
		"nonce":                 func(b *models.Block) { b.Nonce++ },
		"difficulty":            func(b *models.Block) { b.Difficulty++ },
		"version":               func(b *models.Block) { b.Version++ },
		"parent":                func(b *models.Block) { b.PrevHash = strings.Repeat("a", 64) },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			b := original
			b.Transactions = append([]models.Transaction{}, original.Transactions...)
			change(&b)
			if b.CalculateHash() == original.Hash {
				t.Fatal("tampering was not detected")
			}
		})
	}
	a := original
	a.Transactions = []models.Transaction{{ID: "a,b", Data: "x", Timestamp: 1}}
	b := original
	b.Transactions = []models.Transaction{{ID: "a", Data: "x", Timestamp: 1}, {ID: "b", Data: "x", Timestamp: 1}}
	if a.CalculateHash() == b.CalculateHash() {
		t.Fatal("ambiguous transaction encoding")
	}
}

func TestValidationRejectsInvalidChains(t *testing.T) {
	g := models.CreateGenesisBlock(1)
	b := testutil.Block(g, "tx")
	if err := models.ValidateChain([]models.Block{g, b}, 1); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func([]models.Block){
		"tampered genesis": func(c []models.Block) { c[0].Timestamp++ },
		"forged hash":      func(c []models.Block) { c[1].Hash = "x" },
		"no work": func(c []models.Block) {
			for {
				c[1].Nonce++
				c[1].Hash = c[1].CalculateHash()
				if !strings.HasPrefix(c[1].Hash, "0") {
					break
				}
			}
		},
		"wrong parent":          func(c []models.Block) { c[1].PrevHash = strings.Repeat("b", 64) },
		"duplicate transaction": func(c []models.Block) { c[1].Transactions = append(c[1].Transactions, c[1].Transactions[0]) },
		"invalid transaction":   func(c []models.Block) { c[1].Transactions[0].Data = " " },
	} {
		t.Run(name, func(t *testing.T) {
			c := []models.Block{g, b}
			c[1].Transactions = append([]models.Transaction{}, b.Transactions...)
			change(c)
			if models.ValidateChain(c, 1) == nil {
				t.Fatal("invalid chain accepted")
			}
		})
	}
	repeated := testutil.Block(b, "tx")
	if models.ValidateChain([]models.Block{g, b, repeated}, 1) == nil {
		t.Fatal("replay accepted across blocks")
	}
	for _, d := range []int{-1, 0, 5, 65} {
		if models.CheckDifficulty(d) == nil {
			t.Fatalf("accepted difficulty %d", d)
		}
	}
}

func TestProtocolFixtureMatchesBrowser(t *testing.T) {
	raw, err := os.ReadFile("../testdata/protocol-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var chain []models.Block
	if err := json.Unmarshal(raw, &chain); err != nil {
		t.Fatal(err)
	}
	if err := models.ValidateChain(chain, 2); err != nil {
		t.Fatal(err)
	}
	for _, b := range chain {
		if b.Hash != b.CalculateHash() {
			t.Fatal("protocol fixture hash mismatch")
		}
	}
}
