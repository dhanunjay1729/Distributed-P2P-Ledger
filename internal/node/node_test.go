package node

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestConfigValidationAndOverrides(t *testing.T) {
	lookup := func(k string) string {
		return map[string]string{"PORT": "8081", "LEDGER_FILE": "legacy.json", "DIFFICULTY": "1"}[k]
	}
	c, err := ParseConfig([]string{"-port", "8082", "-chain", "current.json"}, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8082 || c.ChainFile != "current.json" || c.Addr != "http://localhost:8082" || c.Difficulty != 1 {
		t.Fatalf("wrong config: %+v", c)
	}
	for _, args := range [][]string{{"-port", "0"}, {"-difficulty", "-1"}, {"-difficulty", "5"}, {"-difficulty", "garbage"}, {"-mine-interval", "0s"}, {"-sync-interval", "-1s"}, {"-addr", "ftp://example.org"}, {"8081"}} {
		if _, err := ParseConfig(args, func(string) string { return "" }); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestNodeStartsAndShutsDown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	c, err := ParseConfig([]string{"-port", strconv.Itoa(port), "-chain", filepath.Join(t.TempDir(), "nested", "chain.json"), "-difficulty", "1"}, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, c) }()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, e := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/healthz")
		if e == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("node never became healthy")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown leaked background workers")
	}
}
