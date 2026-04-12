.PHONY: test test-storage test-api run-node1 run-node2 run-node3 run-local-3 docker-up docker-down docker-logs

test:
	go test ./... -v

test-storage:
	go test ./internal/storage -v

test-api:
	go test ./internal/api -v

run-node1:
	PORT=8081 \
	PEERS_FILE=config/peers/node1.txt \
	NODE_ADDR=http://localhost:8081 \
	LEDGER_FILE=data/ledger_8081.json \
	go run ./node_a

run-node2:
	PORT=8082 \
	PEERS_FILE=config/peers/node2.txt \
	NODE_ADDR=http://localhost:8082 \
	LEDGER_FILE=data/ledger_8082.json \
	go run ./node_a

run-node3:
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

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f