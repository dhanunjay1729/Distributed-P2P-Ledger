test:
	go test ./... -v

test-storage:
	go test ./internal/storage -v

test-api:
	go test ./internal/api -v