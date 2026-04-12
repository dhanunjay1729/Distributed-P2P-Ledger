.PHONY: \
    test test-storage test-api \
    run-node1 run-node2 run-node3 run-local-3 \
    docker-up docker-up-d docker-down docker-logs docker-ps docker-smoke \
    post-tx get-tx get-tx-all

PORT ?= 8081
TX_ID ?= tx1
TX_DATA ?= Alice pays Bob 10
TX_TS ?= 1730000000

test:
    go test ./... -v

test-storage:
    go test ./internal/storage -v

test-api:
    go test ./internal/api -v

run-node1:
    @mkdir -p data
    PORT=8081 \
    PEERS_FILE=config/peers/node1.txt \
    NODE_ADDR=http://localhost:8081 \
    LEDGER_FILE=data/ledger_8081.json \
    go run ./node_a

run-node2:
    @mkdir -p data
    PORT=8082 \
    PEERS_FILE=config/peers/node2.txt \
    NODE_ADDR=http://localhost:8082 \
    LEDGER_FILE=data/ledger_8082.json \
    go run ./node_a

run-node3:
    @mkdir -p data
    PORT=8083 \
    PEERS_FILE=config/peers/node3.txt \
    NODE_ADDR=http://localhost:8083 \
    LEDGER_FILE=data/ledger_8083.json \
    go run ./node_a

run-local-3:
    @mkdir -p data && \
    PORT=8081 PEERS_FILE=config/peers/node1.txt NODE_ADDR=http://localhost:8081 LEDGER_FILE=data/ledger_8081.json go run ./node_a & \
    PORT=8082 PEERS_FILE=config/peers/node2.txt NODE_ADDR=http://localhost:8082 LEDGER_FILE=data/ledger_8082.json go run ./node_a & \
    PORT=8083 PEERS_FILE=config/peers/node3.txt NODE_ADDR=http://localhost:8083 LEDGER_FILE=data/ledger_8083.json go run ./node_a & \
    wait

docker-up:
    docker compose up --build

docker-up-d:
    docker compose up -d --build

docker-down:
    docker compose down

docker-logs:
    docker compose logs -f

docker-ps:
    docker compose ps

docker-smoke:
    $(MAKE) post-tx PORT=8081 TX_ID=tx-smoke TX_DATA="smoke test" TX_TS=$$(date +%s)
    sleep 1
    $(MAKE) get-tx-all

post-tx:
    curl -sS -X POST http://localhost:$(PORT)/transaction \
        -H "Content-Type: application/json" \
        -d "$$(printf '{"id":"%s","data":"%s","timestamp":%s}' "$(TX_ID)" "$(TX_DATA)" "$(TX_TS)")"

get-tx:
    curl -sS http://localhost:$(PORT)/transactions

get-tx-all:
    @for p in 8081 8082 8083 8084 8085 8086; do \
        echo "---- $$p ----"; \
        curl -sS http://localhost:$$p/transactions; \
        echo; \
    done