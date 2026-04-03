package main

import (
	"github.com/gin-gonic/gin"

	"yourmodule/internal/api"
	"yourmodule/internal/storage"
)

func main() {
	router := gin.Default()

	// init storage
	store := storage.NewFileStorage("ledger.json")

	// init handler
	handler := api.NewHandler(store)

	// routes
	router.POST("/transaction", handler.AddTransaction)
	router.GET("/transactions", handler.GetTransactions)

	router.Run(":8080")
}