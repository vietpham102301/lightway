package logger

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ===========================================================================
// Init
// ===========================================================================

func TestInit_DefaultLevel(t *testing.T) {
	Init(Config{Level: "", Format: "text"})
	if !L().Enabled(context.TODO(), slog.LevelInfo) {
		t.Error("expected INFO level to be enabled by default")
	}
}

func TestInit_DebugLevel(t *testing.T) {
	Init(Config{Level: "debug", Format: "text"})
	if !L().Enabled(context.TODO(), slog.LevelDebug) {
		t.Error("expected DEBUG level to be enabled")
	}
}

func TestInit_WarnLevel(t *testing.T) {
	Init(Config{Level: "warn", Format: "text"})
	if L().Enabled(context.TODO(), slog.LevelInfo) {
		t.Error("expected INFO level to be disabled when level is WARN")
	}
	if !L().Enabled(context.TODO(), slog.LevelWarn) {
		t.Error("expected WARN level to be enabled")
	}
}

func TestInit_ErrorLevel(t *testing.T) {
	Init(Config{Level: "error", Format: "text"})
	if L().Enabled(context.TODO(), slog.LevelWarn) {
		t.Error("expected WARN level to be disabled when level is ERROR")
	}
	if !L().Enabled(context.TODO(), slog.LevelError) {
		t.Error("expected ERROR level to be enabled")
	}
}

func TestInit_JSONFormat(t *testing.T) {
	Init(Config{Level: "info", Format: "json"})
	// Just verify it doesn't panic and logger is set
	if L() == nil {
		t.Error("expected logger to be initialized")
	}
}

func TestInit_CaseInsensitive(t *testing.T) {
	Init(Config{Level: "  DEBUG  ", Format: " JSON "})
	if !L().Enabled(context.TODO(), slog.LevelDebug) {
		t.Error("expected DEBUG level with trimmed/case-insensitive input")
	}
}

// ===========================================================================
// Package-level functions (smoke tests)
// ===========================================================================

func TestLogFunctions_NoPanic(t *testing.T) {
	Init(Config{Level: "debug", Format: "text"})
	// These should not panic
	Debug("test debug", "key", "val")
	Info("test info", "key", "val")
	Warn("test warn", "key", "val")
	Error("test error", "key", "val")
}

func TestWith(t *testing.T) {
	Init(Config{Level: "info", Format: "text"})
	l := With("component", "auth")
	if l == nil {
		t.Error("expected non-nil logger from With()")
	}
}

func TestErr(t *testing.T) {
	attr := Err(nil)
	if attr.Key != "err" {
		t.Errorf("expected key 'err', got %q", attr.Key)
	}
}

// ===========================================================================
// HTTPMiddleware
// ===========================================================================

func TestHTTPMiddleware_LogsRequest(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPMiddleware()
	wrapped := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "http request") {
		t.Errorf("expected log to contain 'http request', got %q", output)
	}
	if !strings.Contains(output, "GET") {
		t.Errorf("expected log to contain method 'GET', got %q", output)
	}
	if !strings.Contains(output, "/api/test") {
		t.Errorf("expected log to contain path '/api/test', got %q", output)
	}
	if !strings.Contains(output, "status=200") {
		t.Errorf("expected log to contain 'status=200', got %q", output)
	}
	if !strings.Contains(output, "duration=") {
		t.Errorf("expected log to contain 'duration=', got %q", output)
	}
}

func TestHTTPMiddleware_WarnsOn4xx(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	middleware := HTTPMiddleware()
	wrapped := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/missing", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "level=WARN") {
		t.Errorf("expected WARN level for 404, got %q", output)
	}
}

func TestHTTPMiddleware_ErrorsOn5xx(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	middleware := HTTPMiddleware()
	wrapped := middleware(nextHandler)

	req := httptest.NewRequest("POST", "/crash", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "level=ERROR") {
		t.Errorf("expected ERROR level for 500, got %q", output)
	}
}

func TestHTTPMiddleware_DefaultStatusOK(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))

	// Handler that writes body without explicit WriteHeader
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})

	middleware := HTTPMiddleware()
	wrapped := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "status=200") {
		t.Errorf("expected default status 200, got %q", output)
	}
}
