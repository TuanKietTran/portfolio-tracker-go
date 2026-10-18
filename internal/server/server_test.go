package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
)

func TestValidatedRuntimeConfiguration(t *testing.T) {
	config, err := LoadConfig(func(string) string { return "" })
	if err != nil || config.HTTPAddr != ":8105" || config.CashMode != "local" || config.CostMethod != domain.FIFO {
		t.Fatalf("config=%+v err=%v", config, err)
	}
	for _, env := range []map[string]string{{"HTTP_ADDR": "bad"}, {"HTTP_ADDR": ":0"}, {"HTTP_ADDR": ":70000"}, {"HTTP_ADDR": ":not-a-port"}, {"CASH_MODE": "external"}, {"PAYMENTS_URL": "http://localhost:8102"}, {"DATABASE_URL": "unsupported-test-value"}, {"COST_METHOD": "LIFO"}} {
		if _, err := LoadConfig(func(key string) string { return env[key] }); err == nil {
			t.Fatalf("accepted env=%v", env)
		}
	}
	config, err = LoadConfig(func(key string) string {
		return map[string]string{"HTTP_ADDR": "127.0.0.1:8105", "COST_METHOD": "AVERAGE_COST", "CASH_MODE": "local"}[key]
	})
	if err != nil || config.CostMethod != domain.AverageCost || config.HTTPAddr != "127.0.0.1:8105" {
		t.Fatalf("config=%+v err=%v", config, err)
	}
	server := NewHTTPServer(http.NotFoundHandler())
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 || server.MaxHeaderBytes != 16<<10 {
		t.Fatal("unbounded HTTP server")
	}
}

func TestServeCancellationDrainsActiveRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			<-release
			_, _ = io.WriteString(w, "done")
		}), time.Second)
	}()
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	response := make(chan error, 1)
	go func() {
		resp, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			defer resp.Body.Close()
			body, readErr := io.ReadAll(resp.Body)
			err = readErr
			if string(body) != "done" {
				err = errors.New("response not drained")
			}
		}
		response <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}
	cancel()
	close(release)
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServeErrorsAndForcedShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := Serve(context.Background(), listener, http.NotFoundHandler(), 0); err == nil {
		t.Fatal("accepted unbounded shutdown")
	}
	_ = listener.Close()
	if err := Serve(context.Background(), listener, http.NotFoundHandler(), time.Second); err == nil {
		t.Fatal("closed listener error hidden")
	}
	listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, listener, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { close(entered); <-release }), time.Nanosecond)
	}()
	client := &http.Client{Timeout: 3 * time.Second}
	defer client.CloseIdleConnections()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, _ := client.Get("http://" + listener.Addr().String())
		if resp != nil {
			resp.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown err=%v", err)
	}
	<-clientDone
}
