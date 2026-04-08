package main

import (
	"github.com/gin-gonic/gin"
    "os"
	"p2pledger/internal/api"
	"p2pledger/internal/storage"
	"p2pledger/Gossip_Engine"
	"fmt"
)

func main() {
	if len(os.Args) < 3 {
		panic("Usage: go run main.go <port> <peers_file>")
	}

	port := os.Args[1]
	peersFile := os.Args[2]

	nodeAddr := "http://localhost:" + port

	router := gin.Default()
    // init storage
	store := storage.NewFileStorage("Node -A/ledger_" + port + ".json")
    //neew addition 
	gossipEngine,err := gossip.NewGossipEngine(peersFile, nodeAddr, store)// i need ot give peer directory by defininngalert!!! define 
    fmt.Println(err)
    // init handler
	handler := api.NewHandler(store, gossipEngine)
    // routes
	router.POST("/transaction", handler.AddTransaction)
	router.GET("/transactions", handler.GetTransactions)
	router.POST("/gossip", handler.GossipReceive)

	router.Run(":" + port)
}