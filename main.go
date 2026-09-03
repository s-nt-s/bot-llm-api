package main

import (
	"bot-api/common"
	"bot-api/internal/bot"
	"bot-api/internal/httpapi"
	_ "bot-api/internal/providers"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	address := common.GetEnv("ADDRESS", "127.0.0.1:8080")
	server := &http.Server{
		Addr:    address,
		Handler: httpapi.NewHandler(),
	}

	stopContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("bot-api listening on %s", address)
	serverError := make(chan error, 1)
	go func() {
		serverError <- server.ListenAndServe()
	}()

	select {
	case err := <-serverError:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-stopContext.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("HTTP server shutdown failed: %v", err)
		}
		cancel()
		bot.CloseProviders()
		<-serverError
	}
}
