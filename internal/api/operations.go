package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const MaxRequestBytes int64 = 16 << 10

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	body := errorResponse{RequestID: w.Header().Get("X-Request-ID")}
	body.Error.Code = code
	body.Error.Message = message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(dst)
	if err == nil {
		var trailing any
		if next := decoder.Decode(&trailing); next != io.EOF {
			err = errors.New("exactly one JSON object required")
			if next != nil {
				err = next
			}
		}
	}
	if err == nil {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, 413, "body_too_large", "request body exceeds 16384 bytes")
	} else {
		writeError(w, 400, "invalid_json", "one valid JSON object with known fields required")
	}
	return false
}

type responseStatus struct {
	http.ResponseWriter
	status int
}

func (w *responseStatus) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *responseStatus) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(body)
}
func (w *responseStatus) Unwrap() http.ResponseWriter { return w.ResponseWriter }

type requestMetrics struct{ requests, errors, panics atomic.Uint64 }

// NewRouter owns operational state per instance, without process-global counters.
// Readiness means the configured in-memory simulation is initialized, not that
// external payments, persistence or every requested price is available.
func NewRouter(service *command.PortfolioService, logger *slog.Logger, ready func(context.Context) error) chi.Router {
	if logger == nil {
		logger = slog.Default()
	}
	metrics := &requestMetrics{}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			requestID := req.Header.Get("X-Request-ID")
			if parsed, err := uuid.Parse(requestID); err != nil || parsed == uuid.Nil {
				requestID = uuid.NewString()
			} else {
				requestID = parsed.String()
			}
			w.Header().Set("X-Request-ID", requestID)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			status := &responseStatus{ResponseWriter: w}
			start := time.Now()
			metrics.requests.Add(1)
			defer func() {
				if recovered := recover(); recovered != nil {
					metrics.panics.Add(1)
					// Never log panic payloads, request bodies, keys or financial input.
					logger.Error("request panic", "request_id", requestID)
					if status.status == 0 {
						writeError(status, 500, "internal_error", "internal server error")
					}
				}
				if status.status >= 400 {
					metrics.errors.Add(1)
				}
				logger.Info("http request", "request_id", requestID, "method", req.Method, "status", status.status, "duration_ms", time.Since(start).Milliseconds())
			}()
			next.ServeHTTP(status, req)
		})
	})
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) { writeError(w, 404, "not_found", "route not found") })
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, 405, "method_not_allowed", "method not allowed")
	})
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { writeStatus(w, 200, "ok") })
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if service == nil {
			writeError(w, 503, "not_ready", "local simulation not initialized")
			return
		}
		if ready != nil {
			if err := ready(req.Context()); err != nil {
				writeError(w, 503, "not_ready", "local simulation not ready")
				return
			}
		}
		writeStatus(w, 200, "ready")
	})
	r.Get("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]uint64{"requests": metrics.requests.Load(), "errors": metrics.errors.Load(), "panics": metrics.panics.Load()})
	})
	r.Mount("/v1", NewPortfolioHandlers(service).Routes())
	return r
}

func writeStatus(w http.ResponseWriter, status int, state string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": state, "cash_mode": "local", "store": "memory"})
}
