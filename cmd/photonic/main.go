package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"photonic/internal/cli"
	"photonic/internal/config"
	"photonic/internal/logging"
	"photonic/internal/pipeline"
	"photonic/internal/storage"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger, err := logging.Setup(cfg)
	if err != nil {
		basicLogger := logging.New(cfg.Logging.Level, cfg.Logging.Format)
		basicLogger.Error("failed to setup logging", "error", err)
		logger = basicLogger
	}

	store, err := storage.New(cfg.Paths.DatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	pl := pipeline.New(ctx, cfg.Processing.ParallelJobs, logger, store, &cfg.Alignment, &cfg.Raw)
	defer pl.Stop()

	rootCmd := cli.NewRootCmd(cfg, logger, store, pl)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
