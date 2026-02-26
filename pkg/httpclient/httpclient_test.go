package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// ===========================================================================
// RequestBytes — basic
// ===========================================================================

func TestRequestBytes_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := NewClient()
	body, err := client.RequestBytes(context.Background(), http.MethodGet, server.URL, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(body) != `{"status":"ok"}` {
		t.Errorf("expected body '{\"status\":\"ok\"}', got %q", string(body))
	}
}

func TestRequestBytes_4xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`not found`))
	}))
	defer server.Close()

	client := NewClient()
	body, err := client.RequestBytes(context.Background(), http.MethodGet, server.URL, nil, nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if string(body) != "not found" {
		t.Errorf("expected body 'not found', got %q", string(body))
	}
}

func TestRequestBytes_CustomHeaders(t *testing.T) {
	var receivedAuth string
	var receivedHost string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient()
	_, err := client.RequestBytes(context.Background(), http.MethodGet, server.URL, nil, map[string]string{
		"Authorization": "Bearer test-token",
		"Host":          "custom.host.com",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Errorf("expected Authorization 'Bearer test-token', got %q", receivedAuth)
	}
	if receivedHost != "custom.host.com" {
		t.Errorf("expected Host 'custom.host.com', got %q", receivedHost)
	}
}

func TestRequestBytes_JSONBody(t *testing.T) {
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient()
	payload := map[string]string{"key": "value"}
	_, err := client.RequestBytes(context.Background(), http.MethodPost, server.URL, payload, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", receivedContentType)
	}
}

// ===========================================================================
// Retry Logic
// ===========================================================================

func TestRetry_RetriesOn503(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewClient().WithRetry(RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})

	body, err := client.RequestBytes(context.Background(), http.MethodGet, server.URL, nil, nil)
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %q", string(body))
	}
	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestRetry_ExhaustsAllAttempts(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("down"))
	}))
	defer server.Close()

	client := NewClient().WithRetry(RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})

	_, err := client.RequestBytes(context.Background(), http.MethodGet, server.URL, nil, nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// 1 initial + 2 retries = 3 total
	if attempts.Load() != 3 {
		t.Errorf("expected 3 total attempts, got %d", attempts.Load())
	}
}

func TestRetry_NoRetryOn400(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := NewClient().WithRetry(RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
	})

	_, err := client.RequestBytes(context.Background(), http.MethodGet, server.URL, nil, nil)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if attempts.Load() != 1 {
		t.Errorf("expected 1 attempt (no retry for 400), got %d", attempts.Load())
	}
}

func TestRetry_RetriesOn429(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limited"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewClient().WithRetry(RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
	})

	body, err := client.RequestBytes(context.Background(), http.MethodGet, server.URL, nil, nil)
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %q", string(body))
	}
	if attempts.Load() != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	client := NewClient().WithRetry(RetryConfig{
		MaxRetries: 10,
		BaseDelay:  50 * time.Millisecond,
	})

	_, err := client.RequestBytes(ctx, http.MethodGet, server.URL, nil, nil)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestRetry_CustomShouldRetry(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusConflict) // 409 — not in default RetryOn
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewClient().WithRetry(RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
		ShouldRetry: func(resp *http.Response, err error) bool {
			return err != nil || resp.StatusCode == http.StatusConflict
		},
	})

	body, err := client.RequestBytes(context.Background(), http.MethodGet, server.URL, nil, nil)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("expected 'ok', got %q", string(body))
	}
	if attempts.Load() != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestRetry_NoRetryWithoutConfig(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient() // no WithRetry
	_, err := client.RequestBytes(context.Background(), http.MethodGet, server.URL, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts.Load() != 1 {
		t.Errorf("expected exactly 1 attempt without retry config, got %d", attempts.Load())
	}
}

// ===========================================================================
// RetryConfig
// ===========================================================================

func TestRetryConfig_Backoff(t *testing.T) {
	cfg := RetryConfig{BaseDelay: 100 * time.Millisecond, MaxDelay: 1 * time.Second}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1 * time.Second}, // capped at MaxDelay
		{10, 1 * time.Second},
	}

	for _, tt := range tests {
		got := cfg.backoff(tt.attempt)
		if got != tt.expected {
			t.Errorf("backoff(%d) = %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestRetryConfig_Defaults(t *testing.T) {
	cfg := RetryConfig{}
	cfg.applyDefaults()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.BaseDelay != 500*time.Millisecond {
		t.Errorf("expected BaseDelay 500ms, got %v", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 10*time.Second {
		t.Errorf("expected MaxDelay 10s, got %v", cfg.MaxDelay)
	}
	if len(cfg.RetryOn) != 4 {
		t.Errorf("expected 4 default RetryOn codes, got %d", len(cfg.RetryOn))
	}
}

// ===========================================================================
// Config
// ===========================================================================

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}
	cfg.applyDefaults()

	if cfg.MaxIdleConns != 100 {
		t.Errorf("expected 100, got %d", cfg.MaxIdleConns)
	}
	if cfg.MaxIdleConnsPerHost != 10 {
		t.Errorf("expected 10, got %d", cfg.MaxIdleConnsPerHost)
	}
	if cfg.IdleConnTimeout != 90*time.Second {
		t.Errorf("expected 90s, got %v", cfg.IdleConnTimeout)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected 60s, got %v", cfg.Timeout)
	}
}

// ===========================================================================
// Do
// ===========================================================================

func TestDo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "raw")
	}))
	defer server.Close()

	client := NewClient()
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
