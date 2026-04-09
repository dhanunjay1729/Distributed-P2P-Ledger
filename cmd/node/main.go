package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	// Read server port from env; default to 8000 for local/dev runs.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	// Node ID is used in responses so peers/clients can identify the serving node.
	// what does a serving node mean? It means the node that is currently handling the request and providing responses to clients or peers. 
	// In a distributed P2P ledger system, multiple nodes may be running, and each node can serve requests from clients or other nodes. The NODE_ID helps identify which specific node is responding to a request, which can be useful for debugging, monitoring, or understanding the distribution of requests across the network.
	nodeID := os.Getenv("NODE_ID")

	// Basic endpoint to verify node is running and responding.
	// http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request)  means that when an HTTP request is made 
	// to the root path ("/") of the server, the provided function will be executed. This function takes two 
	// parameters: 'w' which is an http.ResponseWriter used to send responses back to the client, and 'r' 
	// which is an *http.Request containing details about the incoming request. In this case, 
	// the function simply writes a greeting message that includes the node ID to the response, 
	// allowing clients or peers to identify which node is responding to their request.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from %s\n", nodeID)
	})

	// Example endpoint representing node-local transaction view.
	// In a real implementation, this would return actual transaction data relevant to the node's ledger state.
	http.HandleFunc("/transactions", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Transactions from %s\n", nodeID)
	})

	fmt.Println("Starting node:", nodeID, "on port", port)

	// Print peers from environment variable for debugging; in a real implementation, this would be used to establish connections to other nodes.
	peers := os.Getenv("PEERS")
	fmt.Println("Peers:", peers)

	// Start HTTP server with the default ServeMux; blocks until server exits.
	http.ListenAndServe(":"+port, nil)

	
}