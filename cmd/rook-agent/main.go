package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"rook-servicechannel-agent/internal/app"
	"rook-servicechannel-agent/internal/config"
	"rook-servicechannel-agent/internal/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.Args[1:], os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(2)
	}

	logger := logging.New(cfg.LogLevel)
	application := app.New(cfg, logger, os.Stdout)

	if err := application.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "run app: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}
