# Project map

- `cmd/node/main.go`: canonical executable; `node_a/main.go` delegates to it.
- `internal/node/`: flags/environment, HTTP timeouts, signals, startup, shutdown.
- `internal/models/`: transactions, canonical hashes, protocol validation, fork ordering.
- `internal/storage/`: validated startup, atomic saves, live-tip replacement checks.
- `internal/mempool/`: bounded pending messages and reorganization reconciliation.
- `Gossip_Engine/`: coordinated state changes, bounded gossip workers, periodic peer sync.
- `internal/miner/`: cancellable proof of work; no empty-block mining.
- `internal/api/`: JSON validation, submission/status/lookup, peer routes.
- `config/peers/`: Docker DNS peers; `config/local/`: localhost peers.
- `scripts/`: local cluster runner, safe JSON helper, isolated Docker smoke test.
- `docs/`: browser simulation and its pure, tested consensus model.
- `internal/testdata/`: cross-language protocol fixture.
- `data/` and `bin/`: ignored runtime state and build output.
- `.github/workflows/`: backend/simulator/cluster checks and static Pages deployment.
