package api

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"p2pledger/internal/mempool"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
	"p2pledger/internal/testutil"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func setup(t *testing.T) (*gin.Engine, *storage.FileChainStorage, *mempool.Mempool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cs, err := storage.NewFileChainStorage(filepath.Join(t.TempDir(), "chain.json"), 1)
	if err != nil {
		t.Fatal(err)
	}
	mp := mempool.NewMempool()
	return NewRouter(NewHandler(cs, mp)), cs, mp
}
func request(r http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestTransactionLifecycleAndConcurrentDedup(t *testing.T) {
	r, cs, _ := setup(t)
	body := []byte(`{"id":"tx","data":"hello","timestamp":1}`)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request(r, "POST", "/transaction", body)
			if w.Code == 202 {
				accepted.Add(1)
			} else if w.Code != 409 {
				t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted %d duplicates", accepted.Load())
	}
	if w := request(r, "GET", "/transaction/tx", nil); !strings.Contains(w.Body.String(), `"pending"`) {
		t.Fatal(w.Body.String())
	}
	tip, _ := cs.GetLatestBlock()
	b := testutil.Block(tip, "tx")
	raw, _ := json.Marshal(b)
	if w := request(r, "POST", "/block", raw); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	if w := request(r, "GET", "/transaction/tx", nil); !strings.Contains(w.Body.String(), `"confirmed"`) {
		t.Fatal(w.Body.String())
	}
	w := request(r, "GET", "/mempool", nil)
	if !strings.Contains(w.Body.String(), `"count":0`) {
		t.Fatal(w.Body.String())
	}
	if w := request(r, "POST", "/transaction", body); w.Code != 409 {
		t.Fatal("confirmed replay accepted")
	}
	if w := request(r, "GET", "/transaction/missing", nil); w.Code != 404 {
		t.Fatal("missing transaction not reported")
	}
}
func TestInvalidInputAndConsistentEmptyResponses(t *testing.T) {
	r, _, _ := setup(t)
	for name, body := range map[string]string{
		"bad JSON": "{", "string timestamp": `{"id":"x","data":"x","timestamp":"yesterday"}`,
		"unknown field":  `{"id":"x","data":"x","timestamp":1,"extra":true}`,
		"two values":     `{"id":"x","data":"x","timestamp":1} {}`,
		"missing values": `{}`, "invalid utf8": string([]byte{'{', '"', 0xff}),
	} {
		t.Run(name, func(t *testing.T) {
			if w := request(r, "POST", "/transaction", []byte(body)); w.Code != 400 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
	if w := request(r, "POST", "/transaction", bytes.Repeat([]byte("x"), MaxRequestBytes+1)); w.Code != 413 {
		t.Fatalf("oversize status %d", w.Code)
	}
	if w := request(r, "GET", "/transactions", nil); w.Body.String() != "[]" {
		t.Fatal(w.Body.String())
	}
	if w := request(r, "POST", "/block", []byte(`{"hash":"x","prev_hash":"x"}`)); w.Code != 400 {
		t.Fatal("short hash accepted")
	}
	req := httptest.NewRequest("POST", "/transaction", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 415 {
		t.Fatalf("missing content type status %d", w.Code)
	}
}
func TestQueueCapacityIsReported(t *testing.T) {
	r, _, mp := setup(t)
	for i := 0; i < mempool.MaxPending; i++ {
		mp.Add(models.Transaction{ID: strings.Repeat("x", 1) + string(rune(1000+i)), Data: "x", Timestamp: 1})
	}
	if w := request(r, "POST", "/transaction", []byte(`{"id":"overflow","data":"x","timestamp":1}`)); w.Code != 429 {
		t.Fatal(w.Code)
	}
}

type gatedStore struct {
	storage.ChainStorage
	gate             atomic.Bool
	entered, release chan struct{}
}

func (s *gatedStore) GetChain() ([]models.Block, error) {
	chain, err := s.ChainStorage.GetChain()
	if s.gate.CompareAndSwap(true, false) {
		close(s.entered)
		<-s.release
	}
	return chain, err
}
func TestLookupCannotLoseTransactionDuringConfirmation(t *testing.T) {
	_, cs, mp := setup(t)
	gated := &gatedStore{ChainStorage: cs, entered: make(chan struct{}), release: make(chan struct{})}
	h := NewHandler(gated, mp)
	router := NewRouter(h)
	tx := models.Transaction{ID: "transition", Data: "message transition", Timestamp: 1}
	if err := h.Gossip.SubmitTransaction(tx); err != nil {
		t.Fatal(err)
	}
	tip, _ := cs.GetLatestBlock()
	block := testutil.Block(tip, "transition")
	gated.gate.Store(true)
	lookedUp := make(chan *httptest.ResponseRecorder, 1)
	go func() { lookedUp <- request(router, "GET", "/transaction/transition", nil) }()
	<-gated.entered
	committed := make(chan error, 1)
	go func() { committed <- h.Gossip.HandleIncomingBlock(block) }()
	early := false
	select {
	case err := <-committed:
		early = true
		if err != nil {
			t.Error(err)
		}
	case <-time.After(20 * time.Millisecond):
	}
	close(gated.release)
	response := <-lookedUp
	if !early {
		if err := <-committed; err != nil {
			t.Fatal(err)
		}
	}
	if early || response.Code != 200 {
		t.Fatalf("lookup lost a transaction during confirmation: %d %s", response.Code, response.Body.String())
	}
}
