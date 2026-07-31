package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goolify/goolify/internal/config"
	"github.com/goolify/goolify/internal/crypto"
	"github.com/goolify/goolify/internal/db"
	"github.com/goolify/goolify/internal/httpapi"
	"github.com/goolify/goolify/internal/scheduler"
	"github.com/goolify/goolify/internal/sshx"
	"github.com/goolify/goolify/internal/store"
	"github.com/goolify/goolify/internal/terminal"
	"github.com/goolify/goolify/internal/worker"
	"github.com/goolify/goolify/internal/ws"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	switch cmd {
	case "version":
		fmt.Println("goolify", version)
	case "migrate":
		if err := runMigrate(); err != nil {
			fatal(err)
		}
	case "serve":
		if err := runServe(); err != nil {
			fatal(err)
		}
	case "worker":
		if err := runServe(); err != nil { // same process; worker embedded in serve
			fatal(err)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Goolify — open-source self-hosted PaaS

Usage:
  goolify migrate   Run database migrations
  goolify serve     Start API + workers
  goolify worker    Start API + workers (alias)
  goolify version   Print version
`)
}

func fatal(err error) {
	slog.Error(err.Error())
	os.Exit(1)
}

func runMigrate() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.Info("running migrations")
	return db.Migrate(cfg.DatabaseURL)
}

func runServe() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Warn("database not ready, attempting migrate then retry", "err", err)
		if merr := db.Migrate(cfg.DatabaseURL); merr != nil {
			return fmt.Errorf("db connect: %w (migrate: %v)", err, merr)
		}
		pool, err = db.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
	}
	defer pool.Close()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		return err
	}

	box, err := crypto.NewBox(cfg.MasterKey)
	if err != nil {
		return err
	}
	st := store.New(pool, box)
	sshPool := sshx.NewPool()
	defer sshPool.Close()
	hub := ws.NewHub()
	q := worker.NewQueue(st, sshPool, hub, 4)
	q.Start(ctx, 4)
	defer q.Stop()

	sched := scheduler.New(st, sshPool, logger, cfg.DataDir, cfg.DatabaseURL)
	sched.Start(ctx)
	defer sched.Stop()

	termMgr := terminal.NewManager(sshPool, logger)

	api := &httpapi.API{
		Cfg:       cfg,
		Store:     st,
		Queue:     q,
		Hub:       hub,
		Terminals: termMgr,
		Logger:    logger,
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("goolify listening", "addr", cfg.HTTPAddr, "version", version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
