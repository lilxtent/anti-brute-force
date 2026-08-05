package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lilxtent/anti-brute-force/internal/apiclient"
	"github.com/lilxtent/anti-brute-force/internal/cli"
)

const (
	exitOK     = 0
	exitDenied = 1
	exitError  = 2
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	root := cli.NewRootCmd(func(addr string, timeout time.Duration) (cli.Client, error) {
		return apiclient.New(addr, timeout)
	})

	err := root.ExecuteContext(ctx)
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, cli.ErrDenied):
		return exitDenied
	default:
		fmt.Fprintln(os.Stderr, "error:", err)
		return exitError
	}
}
