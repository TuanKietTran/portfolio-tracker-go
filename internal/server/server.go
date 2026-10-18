// Package server owns validated local runtime configuration and HTTP lifecycle.
package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
)

type Config struct {
	HTTPAddr        string
	CashMode        string
	CostMethod      domain.CostMethod
	ShutdownTimeout time.Duration
}

func LoadConfig(getenv func(string) string) (Config, error) {
	c := Config{HTTPAddr: ":8105", CashMode: "local", CostMethod: domain.FIFO, ShutdownTimeout: 5 * time.Second}
	if value := getenv("HTTP_ADDR"); value != "" {
		c.HTTPAddr = value
	}
	_, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return Config{}, errors.New("HTTP_ADDR must be host:port")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, errors.New("HTTP_ADDR requires a port from 1 through 65535")
	}
	if value := getenv("CASH_MODE"); value != "" {
		c.CashMode = value
	}
	if c.CashMode != "local" {
		return Config{}, errors.New("only CASH_MODE=local is implemented; external funding unavailable")
	}
	if getenv("PAYMENTS_URL") != "" {
		return Config{}, errors.New("PAYMENTS_URL is not supported until recovery-safe cash outbox is implemented")
	}
	if getenv("DATABASE_URL") != "" {
		return Config{}, errors.New("DATABASE_URL is not supported by the memory runtime")
	}
	if value := getenv("COST_METHOD"); value != "" {
		c.CostMethod = domain.CostMethod(value)
	}
	if c.CostMethod != domain.FIFO && c.CostMethod != domain.AverageCost {
		return Config{}, errors.New("COST_METHOD must be FIFO or AVERAGE_COST")
	}
	return c, nil
}

func NewHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
}

// Serve drains in-flight requests on cancellation, then forces closure if the
// bounded grace period expires. It always joins the serving goroutine.
func Serve(ctx context.Context, listener net.Listener, handler http.Handler, grace time.Duration) error {
	if grace <= 0 {
		return errors.New("positive shutdown grace period required")
	}
	server := NewHTTPServer(handler)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			_ = server.Close()
		}
		serveErr := <-done
		if err != nil {
			return err
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	}
}
