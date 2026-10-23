// Package node is the single startup implementation for both command wrappers.
package node

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	gossip "p2pledger/Gossip_Engine"
	"p2pledger/internal/api"
	"p2pledger/internal/mempool"
	"p2pledger/internal/miner"
	"p2pledger/internal/models"
	"p2pledger/internal/storage"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	Bind                       string
	Port                       int
	Addr, PeersFile, ChainFile string
	Difficulty                 int
	MineInterval, SyncInterval time.Duration
	Healthcheck                bool
}

func ParseConfig(args []string, lookup func(string) string) (Config, error) {
	env := func(k, defaultValue string) string {
		if v := lookup(k); v != "" {
			return v
		}
		return defaultValue
	}
	fs := flag.NewFlagSet("node", flag.ContinueOnError)
	bind := fs.String("bind", env("BIND_ADDR", "127.0.0.1"), "listening IP address")
	port := fs.String("port", env("PORT", "8080"), "HTTP port")
	addr := fs.String("addr", env("NODE_ADDR", ""), "advertised node URL")
	peers := fs.String("peers", env("PEERS_FILE", ""), "peer URL file (empty for single-node mode)")
	chain := fs.String("chain", env("CHAIN_FILE", env("LEDGER_FILE", "")), "confirmed chain file")
	diff := fs.String("difficulty", env("DIFFICULTY", "2"), "leading zero hex digits (1–4; same on every peer)")
	mine := fs.String("mine-interval", env("MINE_INTERVAL", "10s"), "interval between mining attempts")
	syncInterval := fs.String("sync-interval", env("SYNC_INTERVAL", "5s"), "peer reconciliation interval")
	health := fs.Bool("healthcheck", false, "check the local node and exit")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("use named flags, for example -port 8081 -peers config/local/node1.txt")
	}
	if net.ParseIP(*bind) == nil {
		return Config{}, fmt.Errorf("bind must be an IP address")
	}
	p, err := strconv.Atoi(*port)
	if err != nil || p < 1 || p > 65535 {
		return Config{}, fmt.Errorf("port must be between 1 and 65535")
	}
	d, err := strconv.Atoi(*diff)
	if err != nil {
		return Config{}, fmt.Errorf("difficulty must be an integer")
	}
	if err := models.CheckDifficulty(d); err != nil {
		return Config{}, err
	}
	mi, err := time.ParseDuration(*mine)
	if err != nil || mi < 100*time.Millisecond {
		return Config{}, fmt.Errorf("mining interval must be at least 100ms")
	}
	si, err := time.ParseDuration(*syncInterval)
	if err != nil || si < 100*time.Millisecond {
		return Config{}, fmt.Errorf("sync interval must be at least 100ms")
	}
	if *addr == "" {
		*addr = "http://localhost:" + strconv.Itoa(p)
	}
	u, err := url.Parse(*addr)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.TrimRight(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" {
		return Config{}, fmt.Errorf("invalid node address")
	}
	if *chain == "" {
		*chain = filepath.Join("data", fmt.Sprintf("chain_%d.json", p))
	}
	return Config{Bind: *bind, Port: p, Addr: strings.TrimRight(*addr, "/"), PeersFile: *peers, ChainFile: *chain, Difficulty: d, MineInterval: mi, SyncInterval: si, Healthcheck: *health}, nil
}

func Run(ctx context.Context, c Config) error {
	if c.Healthcheck {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", c.Port))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unhealthy status %d", resp.StatusCode)
		}
		return nil
	}
	cs, err := storage.NewFileChainStorage(c.ChainFile, c.Difficulty)
	if err != nil {
		return fmt.Errorf("initialize chain: %w", err)
	}
	mp := mempool.NewMempool()
	g, err := gossip.NewGossipEngine(c.PeersFile, c.Addr, mp, cs)
	if err != nil {
		return err
	}
	gin.SetMode(gin.ReleaseMode)
	server := &http.Server{Addr: net.JoinHostPort(c.Bind, strconv.Itoa(c.Port)), Handler: api.NewRouter(api.NewHandler(cs, mp, g)),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32 << 10}
	// Bind before starting background workers, so a port conflict leaves no workers.
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	workerCtx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() { defer workers.Done(); g.Run(workerCtx, c.SyncInterval) }()
	go func() {
		defer workers.Done()
		miner.NewMiner(c.Addr, mp, cs, g, c.Difficulty, c.MineInterval).Start(workerCtx)
	}()
	stopped := make(chan error, 1)
	go func() { stopped <- server.Serve(listener) }()
	log.Printf("node %s: chain=%s difficulty=%d", c.Addr, c.ChainFile, c.Difficulty)
	select {
	case <-ctx.Done():
	case err = <-stopped:
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	shutdownErr := server.Shutdown(shutdownCtx)
	shutdownCancel()
	if shutdownErr != nil {
		_ = server.Close()
	}
	cancel()
	workers.Wait()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return shutdownErr
}

func Main() {
	c, err := ParseConfig(os.Args[1:], os.Getenv)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := Run(ctx, c); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
