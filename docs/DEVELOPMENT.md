# Development notes

For the Docker quick start and implementation screenshots, see the [README](../README.md).

## Run without Docker

Requirements: Go 1.25 or later, GNU Make, and Python 3.

```bash
make run-local-3
# Ctrl+C stops all three processes.
```

This uses localhost peers, keeps confirmed chains in `data/local/`, and writes
logs in the same directory. Do not run the native and Docker clusters on the same
ports simultaneously.

For three separate terminals, use `make run-node1`, `make run-node2`, and
`make run-node3`. Those commands use `data/chain_8081.json` and corresponding
files for each port, with peer lists under `config/local/`. Unavailable peers
are retried by later synchronization passes.

A standalone node with no peers:

```bash
go run ./cmd/node -port 8091 -chain data/standalone.json
```

`cmd/node` is the canonical entry point. `node_a` remains a compatibility wrapper
around the same implementation. Named flags replace the old positional arguments.

## What the nodes do

1. Validate a submitted message and atomically reject an already-known ID.
2. Hold it in a bounded mempool and gossip to two randomly selected peers.
3. Mine up to 100 messages into a block, with SHA-256 over all header and
   transaction fields.
4. Validate protocol version, difficulty, proof of work, parent, index, timestamps,
   transaction contents, and duplicate IDs before saving an incoming block.
5. Commit the confirmed chain through a temporary file, file sync, and atomic rename.
6. Cancel stale mining when the chain changes; forward successfully accepted blocks.
7. Reconcile chains and mempools at startup and periodically to repair missed gossip.

All peers must use the same difficulty. Because every block requires the same
target, a longer valid chain represents more required work. Equal-length forks
choose the **smaller tip hash** as a deterministic local-demo tie-break. This
tie-break is a teaching rule, not Bitcoin's fork-selection protocol.

Chain comparison and replacement use the current tip under a lock, so a delayed
peer response cannot roll back newer local progress. Reorganizations return valid
orphaned messages to the mempool and remove newly confirmed messages. An incoming
chain is not adopted if doing so would overflow the pending-message capacity;
the error is logged and later sync passes retry. Nothing is silently discarded.

## Configuration

Flags override environment variables.

| Flag | Environment | Default |
| --- | --- | --- |
| `-port` | `PORT` | `8080` |
| `-bind` | `BIND_ADDR` | `127.0.0.1` (Docker: `0.0.0.0` inside its network) |
| `-addr` | `NODE_ADDR` | `http://localhost:<port>` |
| `-peers` | `PEERS_FILE` | Empty: standalone node |
| `-chain` | `CHAIN_FILE` | `data/chain_<port>.json` |
| `-difficulty` | `DIFFICULTY` | `2` leading zero hexadecimal digits; range 1–4 |
| `-mine-interval` | `MINE_INTERVAL` | `10s`; minimum `100ms` |
| `-sync-interval` | `SYNC_INTERVAL` | `5s`; minimum `100ms` |

`LEDGER_FILE` is accepted as a legacy fallback when `CHAIN_FILE` is unset.
Peer files contain one HTTP(S) origin per line; blank lines and `#` comments
are ignored. Duplicate addresses and the node's own address are filtered out.

Compose accepts `DIFFICULTY`, `MINE_INTERVAL`, and `SYNC_INTERVAL` from the
environment. A difficulty change changes the network's genesis block; use a
separate data directory or new volumes for a different difficulty.

## API

| Method | Path | Result |
| --- | --- | --- |
| POST | `/transaction` | Accept message: 202; duplicate: 409; full queue: 429 |
| GET | `/transaction/:id` | Pending/confirmed status, complete message, confirmations; 404 if absent |
| GET | `/transactions` | All confirmed transactions; empty list is `[]` |
| GET | `/mempool` | `{"count":0,"transactions":[]}` when empty |
| GET | `/chain` | Complete validated chain, including genesis |
| GET | `/status` | Protocol version, difficulty, height, tip hash, pending count |
| GET | `/healthz` | Liveness response |
| POST | `/gossip` | Peer transaction relay; duplicate deliveries are acknowledged |
| POST | `/block` | Peer block relay: 202 accepted, 400 malformed, 409 conflicting tip |

POST requests require `Content-Type: application/json`. Invalid fields, unknown
fields, and trailing JSON are rejected. Timestamp values are integer Unix seconds.
Transaction IDs are unique across the confirmed chain and current mempool.
These endpoints have no authentication; the supported setup is a trusted local
network with host ports bound to loopback.

## Browser simulator

```bash
make simulator
# Open http://localhost:8090; Ctrl+C stops the static server.
```

The existing visualization runs entirely in the browser; it does not control or
read the Go nodes. SHA-256 hashes and proof-of-work checks are real, using the same
versioned encoding as the Go backend. Delays, packet motion, and topology are
animated for teaching. HTTPS or localhost is required for browser cryptography.

- **Send Transaction:** gossip a message, mine it, and observe confirmation.
- **Try Transaction Replay:** resubmit a confirmed ID and observe rejection.
- **Simulate Fork:** mine two real competing blocks on the same parent; watch
  the tie-break and orphan-message recovery.

The demo validates complete candidate chains, so missing intermediate blocks
cannot be skipped. It periodically reconciles peers, bounds the activity log,
and adapts its layout to smaller screens.

## Tests

Backend unit tests need Go. Simulator tests also need Node.js 22 or later.

```bash
make test         # Backend regression tests
make test-race    # Backend tests with the race detector
make check        # Vet, race tests, simulator tests, Compose syntax
make smoke        # Six native processes: convergence, offline recovery, replay, concurrency
make docker-smoke # Six containers: startup, health, restart recovery; always cleaned up
```

`make smoke` uses temporary data and ephemeral ports and stops every process it
starts. `make docker-smoke` uses ports 8081–8086 with a unique Compose project;
run it with the regular cluster stopped. Its cleanup removes only the test
project's containers and temporary volumes, even after failure or Ctrl+C.
The test image and Docker build cache remain available for later runs.

CI runs static checks, race tests, browser-model regression tests, native cluster
tests, and Docker smoke tests. A shared protocol fixture checks that Go and
JavaScript hash identical Unicode and HTML-containing transactions.

## Existing data and limits

Protocol version 1 now hashes complete transactions. Chains written by the older,
ID-only hash implementation are incompatible. The node reports an error and
preserves invalid files instead of rewriting them. Keep old files or volumes as
an archive and select a fresh chain path or new Compose project for the fixed
protocol. Legacy `ledger*.json` samples in the repository are not loaded.

This small demo caps IDs at 128 bytes, messages at 4096 bytes, the mempool at 1000
messages, blocks at 100 transactions, chains at 10,000 blocks / 64 MiB of JSON,
and configured peers at 32. HTTP bodies, timeouts, outgoing workers, retries, and
peer responses are bounded. The chain is kept in memory and rewritten as JSON
when updated; this is appropriate for demonstrations rather than large ledgers.
Only one process should own a particular chain file.

There is no difficulty adjustment, transaction signing, balance accounting,
public-network abuse protection, or Byzantine fault-tolerance guarantee. Do not
expose the peer API publicly or use this project to secure real assets.

## Screenshot source

The README screenshots show captured Docker CLI output and Go API responses from
a local six-container run. The [raw capture](screenshots/local-run.json) includes
the submitted transaction, HTTP responses, node states, and Docker commands.
