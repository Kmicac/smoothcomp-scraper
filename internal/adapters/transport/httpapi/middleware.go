package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	coreerrors "github.com/kmicac/smoothcomp-scraper/internal/core/errors"
	platformconfig "github.com/kmicac/smoothcomp-scraper/internal/platform/config"
	"github.com/kmicac/smoothcomp-scraper/internal/platform/httpx"
)

func correlationMiddleware(header string) muxMiddleware {
	if header == "" {
		header = "X-Correlation-ID"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := strings.TrimSpace(r.Header.Get(header))
			if correlationID == "" {
				correlationID = newCorrelationID()
			}
			w.Header().Set(header, correlationID)
			next.ServeHTTP(w, r.WithContext(httpx.WithCorrelationID(r.Context(), correlationID)))
		})
	}
}

func loggingMiddleware(logger Logger) muxMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(writer, r)
			logger.Info("http request",
				Field("method", r.Method),
				Field("path", r.URL.Path),
				Field("status", writer.status),
				Field("duration_ms", time.Since(start).Milliseconds()),
				Field("correlation_id", httpx.CorrelationIDFromContext(r.Context())),
			)
		})
	}
}

func internalAuthMiddleware(cfg platformconfig.SecurityConfig) muxMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.AllowInsecureAuth {
				next.ServeHTTP(w, r)
				return
			}
			token := extractToken(r, cfg.InternalAuthHeader)
			if token == "" || token != cfg.InternalToken {
				writeError(w, http.StatusUnauthorized, coreerrors.New(coreerrors.CategorySecurity, coreerrors.CodeUnauthorized, "http.auth", false, "missing or invalid internal token", nil))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractToken(r *http.Request, header string) string {
	value := strings.TrimSpace(r.Header.Get(header))
	if value == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return value
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

type muxMiddleware = mux.MiddlewareFunc

func newCorrelationID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return "corr_" + hex.EncodeToString(buf)
}
