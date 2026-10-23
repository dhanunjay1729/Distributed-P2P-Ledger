#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# A unique project name isolates test volumes from the user's regular cluster.
project="p2pledger-smoke-$$"
cleanup() {
    docker compose -p "$project" down --volumes --remove-orphans
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
export DIFFICULTY=1 MINE_INTERVAL=1s SYNC_INTERVAL=1s
docker compose -p "$project" up --build -d --wait --wait-timeout 120
python3 scripts/local_cluster.py --existing
docker compose -p "$project" stop node6
python3 scripts/post_transaction.py --id "offline-$$" --data "Recover after offline period"
python3 - "$$" <<'PYWAIT'
import sys
sys.path.insert(0, "scripts")
from local_cluster import http, wait_for
txid = "offline-" + sys.argv[1]
def confirmed_while_offline():
    chains = [http(f"http://127.0.0.1:{port}", "/chain")[1] for port in range(8081, 8086)]
    return len({chain[-1]["hash"] for chain in chains}) == 1 and all(
        any(tx["id"] == txid for block in chain for tx in block["transactions"]) for chain in chains)
wait_for(confirmed_while_offline, "confirmation with Node 6 offline")
PYWAIT
docker compose -p "$project" start node6
# Check restart recovery without submitting another transaction.
python3 - "$$" <<'PY'
import sys
sys.path.insert(0, "scripts")
from local_cluster import http, wait_for
txid = "offline-" + sys.argv[1]
def caught_up():
    chains = [http(f"http://127.0.0.1:{port}", "/chain")[1] for port in range(8081, 8087)]
    return len({chain[-1]["hash"] for chain in chains}) == 1 and all(
        any(tx["id"] == txid for block in chain for tx in block["transactions"]) for chain in chains)
wait_for(caught_up, "Docker restart catch-up")
print("PASS: Docker restart preserved data and recovered missed blocks")
PY
