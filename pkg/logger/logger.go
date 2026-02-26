package logger

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Level represents log level for configuration
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Config holds logger configuration
type Config struct {
	Level  string
	Format string
}

// Init initializes the global default logger
func Init(cfg Config) {
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
	case LevelDebug:
		level = slog.LevelDebug
	case LevelWarn:
		level = slog.LevelWarn
	case LevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(strings.TrimSpace(cfg.Format)) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// L returns the default logger. Prefer using package-level functions
func L() *slog.Logger {
	return slog.Default()
}

// With returns a logger with the given attributes.
func With(args ...any) *slog.Logger {
	return L().With(args...)
}

// Debug logs at LevelDebug
func Debug(msg string, args ...any) {
	L().Debug(msg, args...)
}

// Info logs at LevelInfo
func Info(msg string, args ...any) {
	L().Info(msg, args...)
}

// Warn logs at LevelWarn
func Warn(msg string, args ...any) {
	L().Warn(msg, args...)
}

// Error logs at LevelError
func Error(msg string, args ...any) {
	L().Error(msg, args...)
}

// Err returns slog attribute for error
func Err(err error) slog.Attr {
	return slog.Any("err", err)
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware returns an HTTP middleware that logs each request with
// method, path, status code, and duration.
func HTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)

			duration := time.Since(start)

			lvl := slog.LevelInfo
			if rec.statusCode >= 500 {
				lvl = slog.LevelError
			} else if rec.statusCode >= 400 {
				lvl = slog.LevelWarn
			}

			L().Log(r.Context(), lvl, "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.statusCode,
				"duration", duration.String(),
			)
		})
	}
}
