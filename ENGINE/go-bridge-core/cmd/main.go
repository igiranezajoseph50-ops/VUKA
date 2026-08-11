// Package main is the VUKA Go bridge engine entrypoint.
//
// Startup order: load config -> connect PostgreSQL -> wire idempotency store,
// adapter registry, ledger service -> start HTTP server with graceful
// shutdown on SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"vuka/go-bridge-core/internal/adapters"
	"vuka/go-bridge-core/internal/api"
	"vuka/go-bridge-core/internal/config"
	bridgegrpc "vuka/go-bridge-core/internal/grpc"
	"vuka/go-bridge-core/internal/grpcpb"
	"vuka/go-bridge-core/internal/idempotency"
	"vuka/go-bridge-core/internal/ledger"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	log.Info("vuka bridge engine starting", "config", cfg.String())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Verify the database is reachable before accepting traffic.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return err
	}
	log.Info("database connected")

	// Wire the adapter registry (Phase 1: simulated MTN Rwanda; Phase 2:
	// simulated M-Pesa Kenya for the RWF -> KES corridor).
	registry := adapters.NewRegistry()
	registry.Register(adapters.NewMTNAdapter(
		cfg.MTNSimDelay,
		cfg.MTNSimFailRate,
		cfg.MTNSimMode,
		cfg.MTNSimTimeout,
	))
	registry.Register(adapters.NewMpesaAdapter(
		cfg.MTNSimDelay,
		cfg.MTNSimFailRate,
		cfg.MTNSimMode,
		cfg.MTNSimTimeout,
	))
	log.Info("adapters registered", "available", registry.Names())

	idem := idempotency.NewStore(pool)
	ledgerSvc := ledger.NewService(pool, idem, registry)
	if cfg.FxRateRWFKES > 0 {
		ledgerSvc.SetDefaultFxRate(cfg.FxRateRWFKES)
	}

	// Seed the SETTLEMENT accounts used by cross-border transfers.
	seedCtx, seedCancel := context.WithTimeout(ctx, 10*time.Second)
	defer seedCancel()
	if err := ledgerSvc.EnsureSettlementAccounts(seedCtx, "RWF", "KES"); err != nil {
		return err
	}
	log.Info("settlement accounts ready", "currencies", []string{"RWF", "KES"})

	// Build the REST handler. When VUKA_WEB_DIR points at the built SPA,
	// serve the dashboard and the API from the same port (single-origin
	// production shape; tunnel-friendly).
	handler := api.NewServer(ledgerSvc, log, cfg.CORSOrigins)
	if cfg.WebDir != "" {
		handler = api.WithWebDir(handler, cfg.WebDir)
		log.Info("serving web dashboard", "dir", cfg.WebDir)
	}

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// gRPC bridge for the Python ISO 20022 gateway (Phase 2).
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}
	gs := grpc.NewServer()
	grpcpb.RegisterBridgeServer(gs, bridgegrpc.NewServer(ledgerSvc))
	reflection.Register(gs)

	// Run both servers in the background and wait for a shutdown signal.
	errCh := make(chan error, 2)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		log.Info("grpc server listening", "addr", cfg.GRPCAddr)
		if err := gs.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	gs.GracefulStop()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("server stopped cleanly")
	return nil
}
