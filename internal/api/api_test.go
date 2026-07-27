/*
This file contains tests for the API handlers of the P2P ledger application. 
It sets up a test router using Gin, 
and defines two test functions: TestAddTransactionAPI and TestGetTransactionsAPI.
The first test checks if a POST request to add a transaction returns a 200 status code, 
while the second test checks if a GET request to retrieve transactions also
returns a 200 status code. The tests use the httptest package to create HTTP requests and record responses.
*/
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"p2pledger/internal/mempool"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	chainStore, _ := storage.NewFileChainStorage("test_api_chain.json")
	defer os.Remove("test_api_chain.json")
	mp := mempool.NewMempool()
	handler := NewHandler(chainStore, mp, nil)

	r := gin.Default()
	r.POST("/transaction", handler.AddTransaction)
	r.GET("/transactions", handler.GetTransactions)

	return r
}

func TestAddTransactionAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	chainStore, _ := storage.NewFileChainStorage(filepath.Join(tmpDir, "chain.json"))
	mp := mempool.NewMempool()
	h := NewHandler(chainStore, mp, nil) // no gossip in unit test

	r := gin.New()
	r.POST("/transaction", h.AddTransaction)

	tx := models.Transaction{
		ID:        time.Now().Format("20060102150405.000000000"), // unique id each run
		Data:      "Alice pays Bob $10",
		Timestamp: time.Now().Unix(),
	}

	body, _ := json.Marshal(tx)
	req := httptest.NewRequest(http.MethodPost, "/transaction", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// In no-gossip mode AddTransaction returns 201 (saved_locally)
	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d, body=%s", http.StatusAccepted, w.Code, w.Body.String())
	}
}

func TestGetTransactionsAPI(t *testing.T) {
	router := setupRouter()

	req, _ := http.NewRequest("GET", "/transactions", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}