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

	"github.com/TuanKietTran/portfolio-tracker-go/internal/api"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, logger *slog.Logger) error {
	config, err := server.LoadConfig(getenv)
	if err != nil {
		return err
	}
	state := repository.NewInMemoryState()
	prices := repository.NewStaticPriceFeed()
	service, err := command.NewPortfolioService(state, prices, nil, config.CostMethod)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", config.HTTPAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	logger.Info("portfolio API listening", "address", listener.Addr().String(), "cash_mode", config.CashMode, "store", "memory", "cost_method", config.CostMethod)
	err = server.Serve(ctx, listener, api.NewRouter(service, logger, nil), config.ShutdownTimeout)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// Kept as the production-router fixture for cross-route regression tests.
func newRouter(service *command.PortfolioService) http.Handler {
	return api.NewRouter(service, slog.Default(), nil)
}
