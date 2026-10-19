// Package server owns validated local runtime configuration and HTTP lifecycle.
package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
)

var activeConnections atomic.Int64

func ActiveConnections() int64 { return activeConnections.Load() }

type Config struct {
	HTTPAddr              string
	CashMode              string
	PaymentsURL           string
	PaymentsSourceAccount string
	PaymentsDestAccount   string
	CostMethod            domain.CostMethod
	ShutdownTimeout       time.Duration
}

func LoadConfig(getenv func(string) string) (Config, error) {
	c := Config{HTTPAddr: "127.0.0.1:8105", CashMode: "local", CostMethod: domain.FIFO, ShutdownTimeout: 5 * time.Second}
	if value := getenv("HTTP_ADDR"); value != "" {
		c.HTTPAddr = value
	}
	host, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return Config{}, errors.New("HTTP_ADDR must be host:port")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return Config{}, errors.New("HTTP_ADDR must bind to loopback")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, errors.New("HTTP_ADDR requires a port from 1 through 65535")
	}
	if value := getenv("CASH_MODE"); value != "" {
		c.CashMode = value
	}
	if c.CashMode != "local" && c.CashMode != "payments" {
		return Config{}, errors.New("CASH_MODE must be local or payments")
	}
	c.PaymentsURL = getenv("PAYMENTS_URL")
	c.PaymentsSourceAccount = getenv("PAYMENTS_SOURCE_ACCOUNT")
	c.PaymentsDestAccount = getenv("PAYMENTS_DEST_ACCOUNT")
	if c.CashMode == "local" && c.PaymentsURL != "" {
		return Config{}, errors.New("PAYMENTS_URL requires CASH_MODE=payments")
	}
	if c.CashMode == "payments" {
		parsed, err := url.ParseRequestURI(c.PaymentsURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Config{}, errors.New("payments mode requires an absolute HTTP PAYMENTS_URL")
		}
		host := parsed.Hostname()
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			return Config{}, errors.New("PAYMENTS_URL must use loopback")
		}
		if strings.TrimSpace(c.PaymentsSourceAccount) == "" || strings.TrimSpace(c.PaymentsDestAccount) == "" || c.PaymentsSourceAccount != strings.TrimSpace(c.PaymentsSourceAccount) || c.PaymentsDestAccount != strings.TrimSpace(c.PaymentsDestAccount) || c.PaymentsSourceAccount == c.PaymentsDestAccount || len(c.PaymentsSourceAccount) > 200 || len(c.PaymentsDestAccount) > 200 {
			return Config{}, errors.New("payments mode requires distinct payment account identifiers of at most 200 bytes")
		}
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
	return &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
		ConnState: func(_ net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				activeConnections.Add(1)
			case http.StateClosed, http.StateHijacked:
				activeConnections.Add(-1)
			}
		},
	}
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
