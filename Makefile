.PHONY: help build test test-storage test-api test-race check run-node1 run-node2 run-node3 run-local-3 smoke docker-up docker-up-d docker-run docker-down docker-clean docker-logs docker-ps docker-smoke post-tx get-tx get-tx-all simulator

PORT ?= 8081
TX_ID ?= tx-$(shell date +%s)
TX_DATA ?= Alice sends Bob a message
TX_TS ?= $(shell date +%s)
export PORT TX_ID TX_DATA TX_TS

help:
	@echo "make docker-up-d   Start six local Docker nodes"
	@echo "make docker-down   Stop nodes; preserve confirmed chains"
	@echo "make run-local-3   Run three native nodes; Ctrl+C stops all"
	@echo "make smoke        Test six native nodes and offline recovery"
	@echo "make docker-smoke Test an isolated Docker cluster and remove it"
	@echo "make check        Run Go checks and simulator regression tests"
	@echo "make simulator    Open http://localhost:8090 in your browser"

build:
	go build -o bin/node ./cmd/node

test:
	go test ./...

test-storage:
	go test ./internal/storage -v

test-api:
	go test ./internal/api -v

test-race:
	go test -race ./... -count=1

check:
	go vet ./...
	go test -race ./... -count=1
	node --test docs/simulator.test.cjs
	docker compose config --quiet

run-node1: build
	./bin/node -port 8081 -peers config/local/node1.txt

run-node2: build
	./bin/node -port 8082 -peers config/local/node2.txt

run-node3: build
	./bin/node -port 8083 -peers config/local/node3.txt

run-local-3: build
	python3 scripts/local_cluster.py --nodes 3

smoke: build
	python3 scripts/local_cluster.py --smoke

docker-up:
	docker compose up --build

docker-up-d:
	docker compose up --build -d --wait

docker-run: docker-up-d

docker-down:
	docker compose down

docker-clean: docker-down

docker-logs:
	docker compose logs -f

docker-ps:
	docker compose ps

docker-smoke:
	bash scripts/docker_smoke.sh

post-tx:
	python3 scripts/post_transaction.py

get-tx:
	curl --fail --silent --show-error http://localhost:$(PORT)/transactions

get-tx-all:
	@for p in 8081 8082 8083 8084 8085 8086; do echo "Node $$p"; curl --fail --silent --show-error http://localhost:$$p/transactions || exit; echo; done

simulator:
	python3 -m http.server 8090 --bind 127.0.0.1 --directory docs
