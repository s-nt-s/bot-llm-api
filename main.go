package main

import (
	"bot-api/common"
	"bot-api/internal/httpapi"
	"log"
	"net/http"
)

func main() {
	address := common.GetEnv("ADDRESS", "127.0.0.1:8080")

	server := &http.Server{
		Addr:    address,
		Handler: httpapi.NewHandler(),
	}

	log.Printf("bot-api listening on %s", address)
	log.Fatal(server.ListenAndServe())
}
