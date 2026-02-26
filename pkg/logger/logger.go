package logger

import (
	"log/slog"
	"os"
	"strings"
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
