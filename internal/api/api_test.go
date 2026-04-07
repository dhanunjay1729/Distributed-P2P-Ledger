package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"p2pledger/internal/storage"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	store := storage.NewFileStorage("test_api.json")
	handler := NewHandler(store)

	r := gin.Default()
	r.POST("/transaction", handler.AddTransaction)
	r.GET("/transactions", handler.GetTransactions)

	return r
}

func TestAddTransactionAPI(t *testing.T) {
	router := setupRouter()

	json := `{
		"id": "tx1",
		"data": "Alice pays Bob",
		"timestamp": 12345
	}`

	req, _ := http.NewRequest("POST", "/transaction", bytes.NewBuffer([]byte(json)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected status 200, got %d", w.Code)
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