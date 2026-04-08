test:
	go test ./... -v

test-storage:
	go test ./internal/storage -v

test-api:
	go test ./internal/api -v

run:
	go run "Node -A/main.go" 8001 "Node -A/peers_8001.txt" & \
	go run "Node -A/main.go" 8002 "Node -A/peers_8002.txt" & \
	go run "Node -A/main.go" 8003 "Node -A/peers_8003.txt" & \
	wait