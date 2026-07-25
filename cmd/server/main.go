package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/lilxtent/anti-brute-force/internal/api"
	"github.com/lilxtent/anti-brute-force/internal/app"
	"github.com/lilxtent/anti-brute-force/internal/config"
)

const shutdownTimeout = 10 * time.Second

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to the config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	svc, closer, err := app.Build(cfg)
	if err != nil {
		log.Fatalf("build: %v", err)
	}
	defer func() {
		if cerr := closer.Close(); cerr != nil {
			log.Printf("close: %v", cerr)
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.Server.HttpPort)
	srv := api.NewServer(svc, addr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("anti-bruteforce listening on %s", addr)
		if serr := srv.ListenAndServe(); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			serverErr <- serr
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("shutting down")
	case serr := <-serverErr:
		log.Printf("server error: %v", serr)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if serr := srv.Shutdown(shutdownCtx); serr != nil {
		log.Printf("graceful shutdown: %v", serr)
	}
}
