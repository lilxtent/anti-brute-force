package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/lilxtent/anti-brute-force/internal/app"
	"github.com/lilxtent/anti-brute-force/internal/config"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to the config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	limiter, closer, err := app.Build(cfg)
	if err != nil {
		log.Fatalf("build: %v", err)
	}
	defer func() {
		if cerr := closer.Close(); cerr != nil {
			log.Printf("close: %v", cerr)
		}
	}()
	_ = limiter

	log.Printf("anti-bruteforce ready (redis=%s, http_port=%d); no HTTP endpoints yet, waiting for signal",
		cfg.Redis.Address, cfg.Server.HttpPort)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("shutting down")
}
