/*
Main entry point for the P2P ledger node that initializes the server, storage, and gossip engine, and defines API routes for transaction handling and gossiping.
what does this file do?
This file initializes the P2P ledger node by reading command-line arguments for the server port and peers file,
setting up the Gin router, initializing the file-based storage for transactions, and creating the gossip engine with the provided peers. 
It then defines API routes for adding transactions, retrieving transactions, and receiving gossip messages, 
and starts the HTTP server to listen for incoming requests on the specified port.

what are these arguements?
The command-line arguments expected by this program are:
1. <port>: This is the port number on which the node will listen for incoming HTTP requests. 
For example, if you want the node to listen on port 8080, you would provide "8080" as this argument.
2. <peers_file>: This is the path to a file that contains a list of peer node addresses. 

The gossip engine will read this file to know which other nodenode_a/main.go 

Main real node server entrypoint (used by Docker build).
Gossip_Engine/gossip.go
Gossip engine: peer selection, dedup, forwarding, retries.

internal/api/handler.go
HTTP handlers for transaction submit/read/gossip receive.
internal/storage/storage.go

Storage interface.
internal/storage/filestorage.go
JSON-file storage implementation.

internal/models/transactions.go
Transaction struct.

config/peers/node1.txt ... node6.txt
Peer lists used in Docker 6-node network.
docker-compose.yml
Runs 6 node services with separate data volumes.

Dockerfile
Builds Go binary from ./node_a.
internal/api/api_test.go, internal/storage/storage_test.go
Unit tests for API and storage.


what is gin router? 
Gin is a web framework written in Go (Golang) that provides a simple and efficient way to build web applications and APIs.
A Gin router is a component of the Gin framework that allows you to define routes for handling HTTP requests. 
You can specify the HTTP method (GET, POST, etc.) and the path for each route, and associate it with a handler function that processes the request and generates a response. 
The router also supports middleware, which can be used to perform actions before or after the main handler is executed, such as logging, authentication, or error handling. 
Overall, the Gin router helps you organize your API endpoints and manage incoming HTTP requests in a clean and efficient manner.

API routes defined in this file:
1. POST /transaction: This route is handled by the AddTransaction method of the Handler struct. 
It allows clients to submit new transactions to the node. The handler will process the incoming transaction data,
check if it already exists, save it to storage, and then gossip it to peers.
2. GET /transactions: This route is handled by the GetTransactions method of the Handler struct.
It allows clients to retrieve all transactions stored on the node. The handler will load transactions from storage and return them as JSON.
3. POST /gossip: This route is handled by the GossipReceive method of the Handler struct. 
It allows the node to receive gossip messages from peers. The handler will process incoming gossip data, 
check if the transaction is new, save it, and gossip it further if necessary.
why is it post? 
*/
package main

import (
	"fmt" 
	"os" // for reading CLI args and env vars
	gossip "p2pledger/Gossip_Engine" 
	"p2pledger/internal/api" 
	"p2pledger/internal/storage"

	"github.com/gin-gonic/gin"
)

// It prioritizes configuration values from CLI arguments, then environment variables, and defaults to a fallback value.
func getArgOrEnv(argIndex int, envKey string, fallback string) string {
	if len(os.Args) > argIndex && os.Args[argIndex] != "" {
		return os.Args[argIndex]
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}

func main() {
	// Priority: CLI args > ENV > fallback
	port := getArgOrEnv(1, "PORT", "8080") 
	peersFile := getArgOrEnv(2, "PEERS_FILE", "peers.txt") 
	nodeAddr := getArgOrEnv(3, "NODE_ADDR", "http://localhost:"+port) 
	ledgerFile := getArgOrEnv(4, "LEDGER_FILE", "node_a/ledger_"+port+".json")

	router := gin.Default() 

	store := storage.NewFileStorage(ledgerFile) // Each node has its own ledger file to avoid conflicts in a multi-node setup. 

	// Initialize gossip engine with peers and storage. The gossip engine will read the list of peers from the specified file and use the provided storage to manage transactions.
	// The transaction storage in the gossip engine is used to keep track of all transactions that the node has seen and processed. This allows the gossip engine to determine whether an incoming transaction is new or has already been seen before. If a transaction is new, the gossip engine will mark it as seen and forward it to other peers. If it has already been seen, the gossip engine will ignore it to prevent redundant processing and forwarding, which helps to reduce network traffic and avoid infinite loops in the gossip protocol.
	gossipEngine, err := gossip.NewGossipEngine(peersFile, nodeAddr, store)
	if err != nil {
		panic("failed to initialize gossip engine: " + err.Error())
	}

	// Create API handler with storage and gossip engine. The API handler will use the storage to manage transactions and the gossip engine to forward new transactions to peers.
	handler := api.NewHandler(store, gossipEngine)

	router.POST("/transaction", handler.AddTransaction)
	router.GET("/transactions", handler.GetTransactions)
	router.POST("/gossip", handler.GossipReceive)

	// Bind on all interfaces for Docker networking.
	if err := router.Run(fmt.Sprintf("0.0.0.0:%s", port)); err != nil {
		panic("failed to run server: " + err.Error())
	}
}