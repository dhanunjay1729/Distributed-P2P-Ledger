# Distributed P2P Ledger (Go)

A simplified peer-to-peer ledger built in Go that propagates transactions across nodes using HTTP gossip and persists each node’s local view to JSON storage.

## Overview

This project demonstrates a lightweight distributed system design:

- **Transaction API** for submitting and reading transactions
- **Gossip propagation** to spread new transactions across peers
- **Per-node persistent storage** using JSON files
- **6-node Docker Compose topology** for local multi-node simulation

Current implementation is focused on **transaction gossip + deduplication + persistence**. The README’s original consensus/longest-chain plan is still roadmap-level and not fully implemented in the current codebase.

## Tech Stack

- **Language:** Go (module: `p2pledger`, Go `1.25.0`)
- **HTTP framework:** Gin (`github.com/gin-gonic/gin`)
- **Containerization:** Docker + Docker Compose
- **Storage:** File-based JSON ledger per node

## Repository Structure

```text
.
├── cmd/node/main.go                 # Minimal standalone net/http demo node
├── node_a/main.go                   # Main runnable P2P node used by Makefile and Docker
├── internal/
│   ├── api/                         # HTTP handlers and API tests
│   │   ├── handler.go
│   │   └── api_test.go
│   ├── models/                      # Domain models
│   │   └── transactions.go
│   └── storage/                     # Storage interface + file storage implementation + tests
│       ├── storage.go
│       ├── filestorage.go
│       └── storage_test.go
├── Gossip_Engine/gossip.go          # Gossip engine (fanout, retries, dedup, forwarding)
├── config/peers/                    # Per-node peer lists (node1.txt ... node6.txt)
├── docker-compose.yml               # 6-node local cluster (node1 ... node6)
├── Dockerfile                       # Multi-stage build for node binary
├── Makefile                         # Local run/test/docker utility commands
└── README.md
```

## How It Works

### 1) Submit Transaction
Client sends a transaction to `POST /transaction`:

```json
{
  "id": "tx3",
  "data": "Alice pays Bob 10",
  "timestamp": 1730000000
}
```

Validation rules (in `internal/api/handler.go`):

- `id` must be non-empty
- `data` must be non-empty
- `timestamp` must be > 0

### 2) Dedup + Persist
The node checks if the transaction already exists (`TransactionExists`) and, if new, saves it to local storage (`SaveTransaction`).

### 3) Gossip Fanout
The gossip engine selects random peers (`fanout = 2` by default) and sends the transaction to each peer via:

- `POST /gossip`

Incoming gossip is handled similarly:

- if already known → ignore
- if new → save locally and forward again

This quickly propagates transactions through the network while limiting redundant rebroadcast loops.

## API Endpoints

Implemented in `internal/api/handler.go`:

- `POST /transaction`
  - Accepts new transaction JSON
  - Triggers gossip when engine is configured
  - Response examples:
    - `202 Accepted` with `{"status":"gossip_started"}`
    - `409 Conflict` if duplicate
    - `400 Bad Request` for invalid input

- `GET /transactions`
  - Returns all locally stored transactions

- `POST /gossip`
  - Receives transaction from a peer node
  - Calls `HandleIncoming` on gossip engine

## Run Locally

### Prerequisites

- Go 1.25+
- Make

### Run 3 local nodes (without Docker)

Use separate terminals:

```bash
make run-node1
make run-node2
make run-node3
```

Or run all three in one command:

```bash
make run-local-3
```

### Submit a transaction

```bash
curl -X POST http://localhost:8081/transaction \
  -H "Content-Type: application/json" \
  -d '{"id":"tx3","data":"test","timestamp":123}'
```

### Read transactions

```bash
curl http://localhost:8081/transactions
```

To query all 6 expected ports quickly:

```bash
make get-tx-all
```

## Run with Docker (6 nodes)

### Start cluster

```bash
make docker-up-d
```

(or foreground logs)

```bash
make docker-up
```

### Check containers

```bash
make docker-ps
```

### Smoke test

```bash
make docker-smoke
```

### Tail logs

```bash
make docker-logs
```

### Stop cluster

```bash
make docker-down
```

## Test

Run all tests:

```bash
make test
```

Run targeted suites:

```bash
make test-storage
make test-api
```

## Environment Variables

Used by `node_a/main.go` + Docker Compose:

- `PORT` – HTTP server port
- `NODE_ADDR` – this node’s base URL (used for self-filtering/logging)
- `PEERS_FILE` – path to peer list file
- `LEDGER_FILE` – path to JSON ledger file

## Notes on Current State

- The **gossip transaction pipeline is implemented** and integrated with API/storage.
- `cmd/node/main.go` is a minimal demo entrypoint and differs from `node_a/main.go` (the practical runtime entrypoint used by scripts/compose).
- Consensus/blockchain conflict resolution from the original problem statement (e.g., previous-hash validation, longest-chain replacement) is **not yet fully present in current implementation**.

## Suggested Next Steps

1. Add explicit block model (`Index`, `PreviousHash`, `CurrentHash`, etc.)
2. Implement block validation endpoint and append rules
3. Add chain sync endpoint for fork resolution
4. Introduce deterministic tests for multi-node conflict scenarios
5. Unify runtime entrypoint (`cmd/node` vs `node_a`) for cleaner architecture
